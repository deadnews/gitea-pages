package main

import (
	"cmp"
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
