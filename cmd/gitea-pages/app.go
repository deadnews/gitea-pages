package main

import (
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/sdk/gitea"
)

// App holds application dependencies.
type App struct {
	client *gitea.Client
	config *Config
}

// NewApp creates a new App with the given configuration.
func NewApp(cfg *Config) (*App, error) {
	client, err := gitea.NewClient(cfg.GiteaServer, gitea.SetToken(cfg.GiteaToken))
	if err != nil {
		return nil, fmt.Errorf("create gitea client: %w", err)
	}
	return &App{client: client, config: cfg}, nil
}

func (app *App) newServer() *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /{owner}/{repo}", handleRepoRedirect)
	mux.HandleFunc("GET /{owner}/{repo}/{path...}", app.handlePages)

	return &http.Server{
		Addr:         app.config.Addr,
		Handler:      Logger(Recoverer(mux)),
		IdleTimeout:  60 * time.Second,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
}
