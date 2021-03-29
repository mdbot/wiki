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
)

//go:embed static
var staticFS embed.FS
var staticFiles fs.FS

// go:embed templates
var templateFS embed.FS
var templateFiles fs.FS

var workDir = flag.String("workdir", "./data", "Working directory")

func main() {
	err := envflag.Parse()
	if err != nil {
		log.Fatalf("Unable to parse flags: %s", err.Error())
	}

	staticFiles, err = GetEmbedOrOSFS("static", staticFS)
	if err != nil {
		log.Fatalf("Unable to get static folder: %s", err.Error())
	}

	templateFiles, err = GetEmbedOrOSFS("templates", templateFS)
	if err != nil {
		log.Fatalf("Unable to get templates folder: %s", err.Error())
	}

	_, err = openOrInit(*workDir)
	if err != nil {
		log.Fatalf("Unable to open working directory: %s", err.Error())
	}

	router := mux.NewRouter()
	router.Use(handlers.ProxyHeaders)
	router.Use(handlers.CompressHandler)
	router.Use(NewLoggingHandler(os.Stdout))
	router.PathPrefix("/view/").Handler(NotFoundHandler(RenderPageHandler(templateFiles, &DummyPageProvider{}), staticFiles))
	router.PathPrefix("/").Handler(NotFoundHandler(http.FileServer(http.FS(staticFiles)), staticFiles))

	log.Print("Starting server.")
	server := http.Server{
		Addr:    ":8080",
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

type DummyPageProvider struct{}

func (*DummyPageProvider) GetPage(title string) (*Page, error) {
	return &Page{
		Content: fmt.Sprintf("Amazing page about **%s** coming soon!", title),
		LastModified: &LogEntry{
			Time:     time.Now(),
			User:     "System",
			Message:  "Magical new page creation",
			ChangeId: "0",
		},
	}, nil
}
