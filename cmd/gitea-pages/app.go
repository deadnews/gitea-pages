package main

import (
	"net/http"
	"strings"
	"time"
)

// App holds application dependencies.
type App struct {
	client *http.Client
	apiURL string
	config *config
}

func newApp(cfg *config) *App {
	return &App{
		client: &http.Client{Timeout: 10 * time.Second},
		apiURL: strings.TrimSuffix(cfg.GiteaServer, "/") + "/api/v1",
		config: cfg,
	}
}

func (app *App) newServer() *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /{owner}/{repo}", handleRepoRedirect)
	mux.HandleFunc("GET /{owner}/{repo}/{path...}", app.handlePages)

	return &http.Server{
		Addr:         app.config.Addr,
		Handler:      logger(recoverer(mux)),
		IdleTimeout:  60 * time.Second,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
}
