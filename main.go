package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/yalue/merged_fs"

	"github.com/csmith/envflag/v2"
	"github.com/gorilla/sessions"
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
	envflag.Parse()

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
	handler := newHandler(
		&Templates{
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
		},
		pm,
		gitBackend,
		renderer,
		userManager,
		siteConfig,
		sessionStore,
		staticFiles,
		os.Stdout,
	)

	log.Print("Starting server.")
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", *httpPort),
		Handler: handler,
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

func newHandler(
	templates *Templates,
	pm *PermissionChecker,
	gitBackend *GitBackend,
	renderer ContentRenderer,
	userManager *config.UserManager,
	siteConfig *config.Site,
	sessionStore sessions.Store,
	static fs.FS,
	logOutput io.Writer,
) http.Handler {
	wiki := newRouter(http.NotFoundHandler())
	wiki.handle("GET /edit/", LowerCaseCanonical(pm.RequireWrite(EditPageHandler(templates, gitBackend))))
	wiki.handle("POST /edit/", LowerCaseCanonical(pm.RequireWrite(SubmitPageHandler(gitBackend))))
	wiki.handle("GET /view/", LowerCaseCanonical(pm.RequireRead(ViewPageHandler(templates, renderer, gitBackend))))
	wiki.handle("GET /history/", LowerCaseCanonical(pm.RequireRead(PageHistoryHandler(templates, gitBackend))))
	wiki.handle("GET /files/view/", LowerCaseCanonical(pm.RequireRead(FileHandler(gitBackend))))
	wiki.handle("GET /files/delete/", LowerCaseCanonical(pm.RequireWrite(DeleteFileConfirmHandler(templates))))
	wiki.handle("POST /files/delete/", LowerCaseCanonical(pm.RequireWrite(DeleteFileHandler(gitBackend))))
	wiki.handle("GET /delete/", LowerCaseCanonical(pm.RequireWrite(DeletePageConfirmHandler(templates))))
	wiki.handle("POST /delete/", LowerCaseCanonical(pm.RequireWrite(DeletePageHandler(gitBackend))))
	wiki.handle("GET /rename/", LowerCaseCanonical(pm.RequireWrite(RenamePageConfirmHandler(gitBackend, templates))))
	wiki.handle("POST /rename/", LowerCaseCanonical(pm.RequireWrite(RenamePageHandler(gitBackend))))
	wiki.handle("GET /revert/", LowerCaseCanonical(pm.RequireWrite(RevertPageConfirmHandler(templates))))
	wiki.handle("POST /revert/", LowerCaseCanonical(pm.RequireWrite(RevertPageHandler(gitBackend))))
	wiki.handle("GET /diff/", LowerCaseCanonical(pm.RequireRead(DiffPageHandler(templates, gitBackend))))
	wiki.handle("GET /api/list", LowerCaseCanonical(pm.RequireRead(ApiListHandler(gitBackend))))
	wiki.handle("GET /wiki/account", LowerCaseCanonical(pm.RequireAccount(AccountHandler(templates))))
	wiki.handle("POST /wiki/account", LowerCaseCanonical(pm.RequireAccount(ModifyAccountHandler(userManager))))
	wiki.handle("GET /wiki/index", LowerCaseCanonical(pm.RequireRead(ListPagesHandler(templates, gitBackend))))
	wiki.handle("GET /wiki/files", LowerCaseCanonical(pm.RequireRead(ListFilesHandler(templates, gitBackend))))
	wiki.handle("GET /wiki/changes", LowerCaseCanonical(pm.RequireRead(RecentChangesHandler(templates, gitBackend))))
	wiki.handle("GET /wiki/changes.xml", LowerCaseCanonical(pm.RequireRead(RecentChangesFeed(templates, gitBackend))))
	wiki.handle("GET /wiki/logo/favicon", LowerCaseCanonical(ServeFavicon(siteConfig)))
	wiki.handle("GET /wiki/logo/main", LowerCaseCanonical(ServeMainLogo(siteConfig)))
	wiki.handle("GET /wiki/logo/dark", LowerCaseCanonical(ServeDarkLogo(siteConfig)))
	wiki.handle("POST /wiki/login", LowerCaseCanonical(LoginHandler(userManager)))
	wiki.handle("POST /wiki/logout", LowerCaseCanonical(LogoutHandler()))
	wiki.handle("GET /wiki/upload", LowerCaseCanonical(pm.RequireWrite(UploadFormHandler(templates))))
	wiki.handle("POST /wiki/upload", LowerCaseCanonical(pm.RequireWrite(UploadHandler(gitBackend))))
	wiki.handle("GET /wiki/search", LowerCaseCanonical(pm.RequireRead(SearchHandler(templates, gitBackend))))
	wiki.handle("GET /wiki/site", LowerCaseCanonical(pm.RequireAdmin(ViewSiteConfigHandler(templates))))
	wiki.handle("POST /wiki/site", LowerCaseCanonical(pm.RequireAdmin(UpdateSiteConfigHandler(siteConfig))))
	wiki.handle("GET /wiki/users", LowerCaseCanonical(pm.RequireAdmin(ManageUsersHandler(templates, userManager))))
	wiki.handle("POST /wiki/users", LowerCaseCanonical(pm.RequireAdmin(ModifyUserHandler(userManager))))

	router := newRouter(wiki.mux)
	router.handle("/{$}", RedirectMainPageHandler())
	router.handle("/view/{$}", RedirectMainPageHandler())
	router.handle("/static/", http.StripPrefix("/static", http.FileServer(http.FS(static))))
	router.handle("/", wiki.mux)

	var handler http.Handler = router.mux
	handler = StripSlashes(handler)
	handler = PageErrorHandler(templates)(handler)
	handler = LoggingHandler(logOutput)(handler)
	handler = SessionHandler(userManager, sessionStore)(handler)
	handler = http.NewCrossOriginProtection().Handler(handler)
	return CleanPath(handler)
}

// router wraps a ServeMux to stop it redirecting "/foo" to "/foo/" when "/foo/" is registered, as StripSlashes
// would immediately redirect back. Such paths are instead passed to the fallback handler.
type router struct {
	mux      *http.ServeMux
	fallback http.Handler
	parents  map[string]bool
}

func newRouter(fallback http.Handler) *router {
	return &router{
		mux:      http.NewServeMux(),
		fallback: fallback,
		parents:  map[string]bool{},
	}
}

func (r *router) handle(pattern string, handler http.Handler) {
	r.mux.Handle(pattern, handler)

	p := pattern[strings.Index(pattern, "/"):]
	p = strings.TrimSuffix(p, "{$}")
	if parent, ok := strings.CutSuffix(p, "/"); ok && parent != "" && !r.parents[parent] {
		r.parents[parent] = true
		r.mux.Handle(parent, r.fallback)
	}
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
