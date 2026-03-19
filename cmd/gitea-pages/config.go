package main

import (
	"errors"
	"os"
)

// Config holds application configuration.
type Config struct {
	GiteaServer string
	GiteaToken  string
	PagesBranch string
	Addr        string
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		GiteaServer: os.Getenv("GITEA_PAGES_SERVER"),
		GiteaToken:  os.Getenv("GITEA_PAGES_TOKEN"),
		PagesBranch: os.Getenv("GITEA_PAGES_BRANCH"),
		Addr:        os.Getenv("GITEA_PAGES_ADDR"),
	}

	if cfg.GiteaServer == "" {
		return nil, errors.New("GITEA_PAGES_SERVER environment variable is required")
	}
	if cfg.GiteaToken == "" {
		return nil, errors.New("GITEA_PAGES_TOKEN environment variable is required")
	}
	if cfg.PagesBranch == "" {
		cfg.PagesBranch = "gh-pages"
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8000"
	}

	return cfg, nil
}
