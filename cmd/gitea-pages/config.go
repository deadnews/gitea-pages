package main

import (
	"cmp"
	"errors"
	"os"
)

type config struct {
	GiteaServer string
	GiteaToken  string
	PagesBranch string
	Addr        string
}

// loadConfig loads configuration from environment variables.
func loadConfig() (*config, error) {
	cfg := &config{
		GiteaServer: os.Getenv("GITEA_PAGES_SERVER"),
		GiteaToken:  os.Getenv("GITEA_PAGES_TOKEN"),
		PagesBranch: cmp.Or(os.Getenv("GITEA_PAGES_BRANCH"), "gh-pages"),
		Addr:        cmp.Or(os.Getenv("GITEA_PAGES_ADDR"), ":8000"),
	}

	if cfg.GiteaServer == "" {
		return nil, errors.New("GITEA_PAGES_SERVER environment variable is required")
	}
	if cfg.GiteaToken == "" {
		return nil, errors.New("GITEA_PAGES_TOKEN environment variable is required")
	}

	return cfg, nil
}
