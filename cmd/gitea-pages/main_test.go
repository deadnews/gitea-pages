package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"code.gitea.io/sdk/gitea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGitea is an in-memory Gitea API stub for tests.
type fakeGitea struct {
	server *httptest.Server
	client *gitea.Client
	files  map[string]string // "owner/repo/branch/path" -> content
}

func newFakeGitea(t *testing.T) *fakeGitea {
	t.Helper()

	files := map[string]string{
		"testorg/testrepo/gh-pages/index.html":        "<html>index</html>",
		"testorg/testrepo/gh-pages/style.css":         "body {}",
		"testorg/testrepo/gh-pages/assets/app.js":     "console.log('hello')",
		"testorg/testrepo/gh-pages/subdir/index.html": "<html>subdir</html>",
		"testorg/testrepo/gh-pages/data":              "raw data",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path

		// GET /api/v1/version - version check
		if urlPath == "/api/v1/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
			return
		}

		// GET /api/v1/repos/{owner}/{repo}/media/{filepath}?ref={ref}
		// Used by SDK GetFile with resolveLFS=true (Gitea >= 1.17)
		prefix := "/api/v1/repos/"
		if strings.HasPrefix(urlPath, prefix) {
			rest := urlPath[len(prefix):]
			parts := strings.SplitN(rest, "/", 4) // [owner, repo, "media", filepath]
			if len(parts) == 4 && parts[2] == "media" {
				ref := r.URL.Query().Get("ref")
				key := fmt.Sprintf("%s/%s/%s/%s", parts[0], parts[1], ref, parts[3])
				if content, ok := files[key]; ok {
					_, _ = w.Write([]byte(content))
					return
				}
			}
		}

		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	client, err := gitea.NewClient(server.URL, gitea.SetToken("test-token"))
	require.NoError(t, err)

	return &fakeGitea{server: server, client: client, files: files}
}

func TestSetupServer(t *testing.T) {
	fg := newFakeGitea(t)
	app := &App{Client: fg.client, PagesBranch: "gh-pages"}
	cfg := &Config{Addr: ":8000"}
	server := setupServer(cfg, app)

	assert.NotNil(t, server)
	assert.NotNil(t, server.Handler)
}

func TestSetupServerNonExistentRoute(t *testing.T) {
	fg := newFakeGitea(t)
	app := &App{Client: fg.client, PagesBranch: "gh-pages"}
	cfg := &Config{Addr: ":8000"}
	server := setupServer(cfg, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/nonexistent", http.NoBody)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestSetupServerMethodNotAllowed(t *testing.T) {
	fg := newFakeGitea(t)
	app := &App{Client: fg.client, PagesBranch: "gh-pages"}
	cfg := &Config{Addr: ":8000"}
	server := setupServer(cfg, app)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"POST to /health", http.MethodPost, "/health"},
		{"PUT to /health", http.MethodPut, "/health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), tt.method, tt.path, http.NoBody)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			server.Handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		})
	}
}

// TestIntegration uses a real HTTP server and client to verify end-to-end
// behavior including redirect chains that httptest.NewRecorder cannot exercise.
func TestIntegration(t *testing.T) {
	fg := newFakeGitea(t)

	app := &App{Client: fg.client, PagesBranch: "gh-pages"}
	cfg := &Config{Addr: ":0"}
	srv := httptest.NewServer(setupServer(cfg, app).Handler)
	t.Cleanup(srv.Close)

	client := srv.Client()

	tests := []struct {
		name       string
		method     string
		path       string
		wantURL    string // expected final URL path after redirects
		wantStatus int
		wantBody   string
		wantHeader map[string]string
	}{
		{
			name:       "repo root redirect chain serves index",
			path:       "/testorg/testrepo",
			wantURL:    "/testorg/testrepo/",
			wantStatus: http.StatusOK,
			wantBody:   "<html>index</html>",
			wantHeader: map[string]string{"Content-Type": "text/html; charset=utf-8"},
		},
		{
			name:       "directory redirect chain serves index",
			path:       "/testorg/testrepo/subdir",
			wantURL:    "/testorg/testrepo/subdir/",
			wantStatus: http.StatusOK,
			wantBody:   "<html>subdir</html>",
		},
		{
			name:       "query string preserved through redirect",
			path:       "/testorg/testrepo/subdir?v=123",
			wantURL:    "/testorg/testrepo/subdir/",
			wantStatus: http.StatusOK,
			wantBody:   "<html>subdir</html>",
		},
		{
			name:       "serves file directly without redirect",
			path:       "/testorg/testrepo/style.css",
			wantURL:    "/testorg/testrepo/style.css",
			wantStatus: http.StatusOK,
			wantBody:   "body {}",
			wantHeader: map[string]string{"Content-Type": "text/css; charset=utf-8"},
		},
		{
			name:       "serves nested file",
			path:       "/testorg/testrepo/assets/app.js",
			wantURL:    "/testorg/testrepo/assets/app.js",
			wantStatus: http.StatusOK,
			wantBody:   "console.log('hello')",
			wantHeader: map[string]string{"Content-Type": "text/javascript; charset=utf-8"},
		},
		{
			name:       "HEAD returns headers without body",
			method:     http.MethodHead,
			path:       "/testorg/testrepo/style.css",
			wantURL:    "/testorg/testrepo/style.css",
			wantStatus: http.StatusOK,
			wantHeader: map[string]string{"Content-Type": "text/css; charset=utf-8"},
		},
		{
			name:       "missing file returns 404",
			path:       "/testorg/testrepo/missing.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing repo returns 404",
			path:       "/testorg/norepo/",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}

			req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+tt.path, http.NoBody)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.wantURL != "" {
				assert.Equal(t, tt.wantURL, resp.Request.URL.Path)
			}

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, string(body))
			}

			if tt.method == http.MethodHead {
				assert.Empty(t, body)
			}

			for k, v := range tt.wantHeader {
				assert.Equal(t, v, resp.Header.Get(k))
			}
		})
	}
}
