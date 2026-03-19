package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApp(t *testing.T) {
	t.Run("creates app with valid config", func(t *testing.T) {
		fg := newFakeGitea(t)
		cfg := &Config{
			GiteaServer: fg.server.URL,
			GiteaToken:  "test-token",
			PagesBranch: "gh-pages",
		}

		app, err := NewApp(cfg)

		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Client)
		assert.Equal(t, "gh-pages", app.PagesBranch)
	})

	t.Run("propagates pages branch from config", func(t *testing.T) {
		fg := newFakeGitea(t)
		cfg := &Config{
			GiteaServer: fg.server.URL,
			GiteaToken:  "test-token",
			PagesBranch: "custom-branch",
		}

		app, err := NewApp(cfg)

		require.NoError(t, err)
		assert.Equal(t, "custom-branch", app.PagesBranch)
	})

	t.Run("returns error for invalid server URL", func(t *testing.T) {
		cfg := &Config{
			GiteaServer: "://invalid-url",
			GiteaToken:  "test-token",
			PagesBranch: "gh-pages",
		}

		app, err := NewApp(cfg)

		require.Error(t, err)
		assert.Nil(t, app)
	})
}
