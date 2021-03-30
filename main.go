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
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/kouhin/envflag"
	"github.com/yalue/merged_fs"
)

//go:embed static templates
var embeddedFiles embed.FS

var staticFiles fs.FS
var templateFiles fs.FS

var workDir = flag.String("workdir", "./data", "Working directory")
var username = flag.String("authusername", "", "username protecting edit page")
var password = flag.String("authpassword", "", "password protecting edit page")
var realm = flag.String("authrealm", "", "realm protecting edit page.  If unset no auth will be used")
var mainPage = flag.String("mainpage", "MainPage", "Title of the main page for the wiki")
var codeStyle = flag.String("codestyle", "monokai", "Style to use for code highlighting. See https://github.com/alecthomas/chroma/tree/master/styles")
var httpPort = flag.Int("httpport", 8080, "HTTP server port")

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

	if err := gitBackend.CreateDefaultMainPage(); err != nil {
		log.Fatalf("Unable to create default main page: %s", err.Error())
	}

	authHandler := BasicAuthFromEnv()
	router := mux.NewRouter()
	router.Use(handlers.ProxyHeaders)
	router.Use(handlers.CompressHandler)
	router.Use(NewLoggingHandler(os.Stdout))
	router.Path("/view/").Handler(RedirectMainPageHandler())
	router.Path("/").Handler(RedirectMainPageHandler())
	router.PathPrefix("/edit/").Handler(NotFoundHandler(EditPageHandler(templateFiles, gitBackend), templateFiles)).Methods(http.MethodGet)
	router.PathPrefix("/edit/").Handler(authHandler(NotFoundHandler(SubmitPageHandler(gitBackend), templateFiles))).Methods(http.MethodPost)
	router.PathPrefix("/view/").Handler(RenderPageHandler(templateFiles, gitBackend)).Methods(http.MethodGet)
	router.PathPrefix("/wiki/index").Handler(ListPagesHandler(templateFiles, gitBackend)).Methods(http.MethodGet)
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
