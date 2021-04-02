package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"github.com/gorilla/csrf"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/kouhin/envflag"
	"github.com/mdbot/wiki/config"
	"github.com/mdbot/wiki/markdown"
	"github.com/yalue/merged_fs"
)

//go:embed static templates
var embeddedFiles embed.FS

var staticFiles fs.FS
var templateFiles fs.FS

var workDir = flag.String("workdir", "./data", "Working directory")
var username = flag.String("username", "", "username for initial account")
var password = flag.String("password", "", "password for initial account")
var mainPage = flag.String("mainpage", "MainPage", "Title of the main page for the wiki")
var codeStyle = flag.String("codestyle", "monokai", "Style to use for code highlighting. See https://github.com/alecthomas/chroma/tree/master/styles")
var httpPort = flag.Int("httpport", 8080, "HTTP server port")
var configKey = flag.String("key", "", "Key to use to encrypt config data (32 byes, hex encoded, e.g. from `openssl rand -hex 32`)")
var requireAuthForWrites = flag.Bool("authenticated-writes", true, "Whether to require authentication to make changes to pages/files")
var requireAuthForReads = flag.Bool("authenticated-reads", false, "Whether to require authentication to read pages/files")

func main() {
	err := envflag.Parse()
	if err != nil {
		log.Fatalf("Unable to parse flags: %s", err.Error())
	}

	staticFs, _ := fs.Sub(embeddedFiles, "static")
	staticFiles = merged_fs.NewMergedFS(os.DirFS("static"), staticFs)

	templateFs, _ := fs.Sub(embeddedFiles, "templates")
	templateFiles = merged_fs.NewMergedFS(os.DirFS("templates"), templateFs)

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

	if err := gitBackend.CreateDefaultMainPage(); err != nil {
		log.Fatalf("Unable to create default main page: %s", err.Error())
	}

	sessionStore := sessions.NewCookieStore(secrets.SessionKey)

	renderer := markdown.NewRenderer(gitBackend, *codeStyle)

	wikiRouter := mux.NewRouter()
	wikiRouter.Use(LowerCaseCanonical)
	wikiRouter.Use(CheckAuthentication(*requireAuthForReads, *requireAuthForWrites))

	wikiRouter.PathPrefix("/edit/").Handler(EditPageHandler(templateFiles, gitBackend)).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/edit/").Handler(SubmitPageHandler(gitBackend)).Methods(http.MethodPost)
	wikiRouter.PathPrefix("/view/").Handler(RenderPageHandler(templateFiles, renderer, gitBackend)).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/history/").Handler(PageHistoryHandler(templateFiles, gitBackend)).Methods(http.MethodGet)
	wikiRouter.PathPrefix("/file/").Handler(FileHandler(gitBackend)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/index").Handler(ListPagesHandler(templateFiles, gitBackend)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/files").Handler(ListFilesHandler(templateFiles, gitBackend)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/login").Handler(LoginHandler(userManager)).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/logout").Handler(LogoutHandler()).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/upload").Handler(UploadFormHandler(templateFiles)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/upload").Handler(UploadHandler(gitBackend)).Methods(http.MethodPost)
	wikiRouter.Path("/wiki/users").Handler(ManageUsersHandler(templateFiles, userManager)).Methods(http.MethodGet)
	wikiRouter.Path("/wiki/users").Handler(ModifyUserHandler(userManager)).Methods(http.MethodPost)

	router := mux.NewRouter()

	router.Use(handlers.CompressHandler)
	router.Use(csrf.Protect(secrets.CsrfKey, csrf.SameSite(csrf.SameSiteStrictMode), csrf.Path("/")))
	router.Use(SessionHandler(userManager, sessionStore))
	router.Use(NewLoggingHandler(os.Stdout))
	router.Use(PageErrorHandler(templateFs))

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
