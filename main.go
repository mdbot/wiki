package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/gorilla/csrf"
	"github.com/yalue/merged_fs"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/kouhin/envflag"
	"github.com/mdbot/wiki/config"
	"github.com/mdbot/wiki/markdown"
)

//go:embed resources/static resources/templates resources/content/*
var embeddedFiles embed.FS

var staticFiles fs.FS
var templateFiles fs.FS

// Provided dynamically in the release process
var version string

var workDir = flag.String("workdir", "./data", "Working directory")
var username = flag.String("username", "", "username for initial account")
var password = flag.String("password", "", "password for initial account")
var mainPage = flag.String("mainpage", "MainPage", "Title of the main page for the wiki")
var codeStyle = flag.String("codestyle", "monokai", "Style to use for code highlighting. See https://github.com/alecthomas/chroma/tree/master/styles")
var httpPort = flag.Int("httpport", 8080, "HTTP server port")
var configKey = flag.String("key", "", "Key to use to encrypt config data (32 byes, hex encoded, e.g. from `openssl rand -hex 32`)")
var requireAuthForWrites = flag.Bool("authenticated-writes", true, "Whether to require authentication to make changes to pages/files")
var requireAuthForReads = flag.Bool("authenticated-reads", false, "Whether to require authentication to read pages/files")
var dangerousHtml = flag.Bool("allow-dangerous-html", false, "Whether to allow dangerous HTML such as script tags")

func main() {
	err := envflag.Parse()
	if err != nil {
		log.Fatalf("Unable to parse flags: %s", err.Error())
	}

	if *dangerousHtml && !*requireAuthForWrites {
		log.Fatal("Refusing to start with dangerous HTML and anonymous writes enabled")
	}

	initFileSystem()

	gitBackend, err := NewGitBackend(*workDir)
	if err != nil {
		log.Fatalf("Unable to open working directory: %s", err.Error())
	}

	configStore := config.NewStore(gitBackend, *configKey)
	userManager, err := config.NewUserManager(configStore)
	if err != nil {
		log.Fatalf("Unable to create user manager: %v", err.Error())
	}

	if userManager.Empty() && *username != "" && *password != "" {
		_ = userManager.AddUser(*username, *password, "System")
	}

	secrets, err := config.LoadSecrets(configStore)
	if err != nil {
		log.Fatalf("Unable to initialise secrets: %v", err.Error())
	}

	if err := createDefaultPages(gitBackend); err != nil {
		log.Fatalf("Unable to create default pages: %s", err.Error())
	}

	siteConfig, err := config.LoadSite(configStore)
	if err != nil {
		log.Fatalf("Unable to load site config: %v", err)
	}

	pm := &PermissionChecker{
		requireAuthForWrites: *requireAuthForWrites,
		requireAuthForReads:  *requireAuthForReads,
	}

	sessionStore := sessions.NewCookieStore(secrets.SessionKey)
	renderer := markdown.NewRenderer(gitBackend, *dangerousHtml, *codeStyle)
	templates := &Templates{
		fs:         templateFiles,
		siteConfig: siteConfig,
		checker:    pm,
		version:    version,
		sidebarProvider: func() string {
			p, err := gitBackend.GetPage("_sidebar")
			if err != nil {
				log.Printf("Unable to load sidebar content: %v", err)
				return "Error loading sidebar"
			}

			s, err := renderer.Render(p.Content)
			if err != nil {
				log.Printf("Unable to render sidebar content: %v", err)
				return "Error rendering sidebar"
			}

			return s
		},
	}

	wikiRouter := mux.NewRouter()
	wikiRouter.Use(LowerCaseCanonical)

	wikiRouter.PathPrefix("/edit/").Handler(pm.RequireWrite(EditPageHandler(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/edit/").Handler(pm.RequireWrite(SubmitPageHandler(gitBackend))).Methods(http.MethodPost)
	wikiRouter.PathPrefix("/view/").Handler(pm.RequireRead(ViewPageHandler(templates, renderer, gitBackend))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/history/").Handler(pm.RequireRead(PageHistoryHandler(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/files/view/").Handler(pm.RequireRead(FileHandler(gitBackend))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/files/delete/").Handler(pm.RequireWrite(DeleteFileConfirmHandler(templates))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/files/delete/").Handler(pm.RequireWrite(DeleteFileHandler(gitBackend))).Methods(http.MethodPost)
	wikiRouter.PathPrefix("/delete/").Handler(pm.RequireWrite(DeletePageConfirmHandler(templates))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/delete/").Handler(pm.RequireWrite(DeletePageHandler(gitBackend))).Methods(http.MethodPost)
	wikiRouter.PathPrefix("/rename/").Handler(pm.RequireWrite(RenamePageConfirmHandler(gitBackend, templates))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/rename/").Handler(pm.RequireWrite(RenamePageHandler(gitBackend))).Methods(http.MethodPost)
	wikiRouter.PathPrefix("/revert/").Handler(pm.RequireWrite(RevertPageConfirmHandler(templates))).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/revert/").Handler(pm.RequireWrite(RevertPageHandler(gitBackend))).Methods(http.MethodPost)
	wikiRouter.PathPrefix("/diff/").Handler(pm.RequireRead(DiffPageHandler(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.Path("/api/list").Handler(pm.RequireRead(ApiListHandler(gitBackend))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/account").Handler(pm.RequireAccount(AccountHandler(templates))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/account").Handler(pm.RequireAccount(ModifyAccountHandler(userManager))).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/index").Handler(pm.RequireRead(ListPagesHandler(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/files").Handler(pm.RequireRead(ListFilesHandler(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/changes").Handler(pm.RequireRead(RecentChangesHandler(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/changes.xml").Handler(pm.RequireRead(RecentChangesFeed(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/logo/favicon").Handler(ServeFavicon(siteConfig)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/logo/main").Handler(ServeMainLogo(siteConfig)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/logo/dark").Handler(ServeDarkLogo(siteConfig)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/login").Handler(LoginHandler(userManager)).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/logout").Handler(LogoutHandler()).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/upload").Handler(pm.RequireWrite(UploadFormHandler(templates))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/upload").Handler(pm.RequireWrite(UploadHandler(gitBackend))).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/search").Handler(pm.RequireRead(SearchHandler(templates, gitBackend))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/site").Handler(pm.RequireAdmin(ViewSiteConfigHandler(templates))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/site").Handler(pm.RequireAdmin(UpdateSiteConfigHandler(siteConfig))).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/users").Handler(pm.RequireAdmin(ManageUsersHandler(templates, userManager))).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/users").Handler(pm.RequireAdmin(ModifyUserHandler(userManager))).Methods(http.MethodPost)

	router := mux.NewRouter()

	router.Use(csrf.Protect(secrets.CsrfKey, csrf.SameSite(csrf.SameSiteStrictMode), csrf.Path("/")))
	router.Use(SessionHandler(userManager, sessionStore))
	router.Use(LoggingHandler(os.Stdout))
	router.Use(PageErrorHandler(templates))
	router.Use(StripSlashes)

	router.Path("/").Handler(RedirectMainPageHandler())
	router.Path("/view/").Handler(RedirectMainPageHandler())
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static", http.FileServer(http.FS(staticFiles))))
	router.NewRoute().Handler(wikiRouter)

	log.Print("Starting server.")
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", *httpPort),
		Handler: router,
	}
	go func() {
		_ = server.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Unable to shutdown: %s", err.Error())
	}
	log.Print("Finishing server.")
}

func initFileSystem() {
	staticFs, _ := fs.Sub(embeddedFiles, "resources/static")
	staticFiles = merged_fs.NewMergedFS(os.DirFS("resources/static"), staticFs)

	templateFs, _ := fs.Sub(embeddedFiles, "resources/templates")
	templateFiles = merged_fs.NewMergedFS(os.DirFS("resources/templates"), templateFs)
}

func createDefaultPages(b *GitBackend) error {
	files, err := embeddedFiles.ReadDir("resources/content")
	if err != nil {
		return err
	}

	for i := range files {
		if !files[i].IsDir() {
			name := strings.TrimSuffix(files[i].Name(), ".md")
			if name == "mainpage" {
				name = *mainPage
			}

			_, err := b.GetPage(name)
			if err != nil {
				log.Printf("Adding default file: %s", name)

				bs, err := embeddedFiles.ReadFile(path.Join("resources", "content", files[i].Name()))
				if err != nil {
					return err
				}

				if err := b.PutPage(name, bs, "system", "Creating default page"); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
