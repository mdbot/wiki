package main

import (
	"context"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/greboid/wiki/markdown"
	"github.com/kouhin/envflag"
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

	var configStore ConfigStore
	keyBytes, _ := hex.DecodeString(*configKey)
	if len(keyBytes) == 32 {
		var key [32]byte
		copy(key[:], keyBytes)
		configStore = &EncryptedConfigStore{
			key:     key,
			backend: gitBackend,
		}
	} else {
		configStore = &DummyConfigStore{}
	}

	userManager, err := NewUserManager(configStore)
	if err != nil {
		log.Fatalf("Unable to create user manager: %v", err.Error())
	}

	if userManager.Empty() && *username != "" && *password != "" {
		_ = userManager.AddUser("System", *username, *password)
	}

	if err := gitBackend.CreateDefaultMainPage(); err != nil {
		log.Fatalf("Unable to create default main page: %s", err.Error())
	}

	sessionStore := sessions.NewCookieStore(userManager.sessionKey)

	renderer := markdown.NewRenderer(gitBackend, *codeStyle)
	router := mux.NewRouter()
	router.Use(handlers.ProxyHeaders)
	router.Use(handlers.CompressHandler)
	router.Use(SessionHandler(userManager, sessionStore))
	router.Use(NewLoggingHandler(os.Stdout))
	router.Use(LowerCaseCanonical)
	router.Path("/view/").Handler(RedirectMainPageHandler())
	router.Path("/").Handler(RedirectMainPageHandler())
	router.PathPrefix("/edit/").Handler(NotFoundHandler(EditPageHandler(templateFiles, gitBackend), templateFiles)).Methods(http.MethodGet)
	router.PathPrefix("/edit/").Handler(RequireAnyUser(NotFoundHandler(SubmitPageHandler(gitBackend), templateFiles))).Methods(http.MethodPost)
	router.PathPrefix("/view/").Handler(RenderPageHandler(templateFiles, renderer, gitBackend)).Methods(http.MethodGet)
	router.PathPrefix("/wiki/index").Handler(ListPagesHandler(templateFiles, gitBackend)).Methods(http.MethodGet)
	router.PathPrefix("/wiki/login").Handler(LoginHandler(sessionStore, userManager)).Methods(http.MethodPost)
	router.PathPrefix("/").Handler(NotFoundHandler(http.FileServer(http.FS(staticFiles)), templateFiles))

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
