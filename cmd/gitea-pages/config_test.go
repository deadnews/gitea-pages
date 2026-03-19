package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name:    "missing GITEA_PAGES_SERVER",
			envVars: map[string]string{"GITEA_PAGES_TOKEN": "tok"},
			wantErr: true,
		},
		{
			name:    "missing GITEA_PAGES_TOKEN",
			envVars: map[string]string{"GITEA_PAGES_SERVER": "https://gitea.example.com"},
			wantErr: true,
		},
		{
			name: "valid config with defaults",
			envVars: map[string]string{
				"GITEA_PAGES_SERVER": "https://gitea.example.com",
				"GITEA_PAGES_TOKEN":  "test-token",
			},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "https://gitea.example.com", cfg.GiteaServer)
				assert.Equal(t, "test-token", cfg.GiteaToken)
				assert.Equal(t, "gh-pages", cfg.PagesBranch)
				assert.Equal(t, ":8000", cfg.Addr)
			},
		},
		{
			name: "custom pages branch and addr",
			envVars: map[string]string{
				"GITEA_PAGES_SERVER": "https://gitea.example.com",
				"GITEA_PAGES_TOKEN":  "test-token",
				"GITEA_PAGES_BRANCH": "pages",
				"GITEA_PAGES_ADDR":   ":9000",
			},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "pages", cfg.PagesBranch)
				assert.Equal(t, ":9000", cfg.Addr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITEA_PAGES_SERVER", "")
			t.Setenv("GITEA_PAGES_TOKEN", "")
			t.Setenv("GITEA_PAGES_BRANCH", "")
			t.Setenv("GITEA_PAGES_ADDR", "")

			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := LoadConfig()

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}
