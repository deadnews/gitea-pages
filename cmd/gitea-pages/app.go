package main

import (
	"fmt"

	"code.gitea.io/sdk/gitea"
)

// App holds application dependencies.
type App struct {
	Client      *gitea.Client
	PagesBranch string
}

// NewApp creates a new App with the given configuration.
func NewApp(cfg *Config) (*App, error) {
	client, err := gitea.NewClient(cfg.GiteaServer, gitea.SetToken(cfg.GiteaToken))
	if err != nil {
		return nil, fmt.Errorf("creating gitea client: %w", err)
	}
	return &App{Client: client, PagesBranch: cfg.PagesBranch}, nil
}
