package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"code.gitea.io/sdk/gitea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGitea is an in-memory Gitea API stub for tests.
type fakeGitea struct {
	server       *httptest.Server
	client       *gitea.Client
	fileRequests atomic.Int32
}

func newFakeGitea(t *testing.T) *fakeGitea {
	t.Helper()

	fg := &fakeGitea{}
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
				fg.fileRequests.Add(1)
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

	fg.server = server
	fg.client = client
	return fg
}

func (f *fakeGitea) newApp() *App {
	return &App{Client: f.client, Config: &Config{PagesBranch: "gh-pages", Addr: ":8000"}}
}

func TestNewServerNonExistentRoute(t *testing.T) {
	app := newFakeGitea(t).newApp()
	server := app.newServer()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/nonexistent", http.NoBody)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestNewServerMethodNotAllowed(t *testing.T) {
	app := newFakeGitea(t).newApp()
	server := app.newServer()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "/health", http.NoBody)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

// TestIntegration uses a real HTTP server and client to verify end-to-end
// behavior including redirect chains that httptest.NewRecorder cannot exercise.
func TestIntegration(t *testing.T) {
	app := newFakeGitea(t).newApp()
	app.Config.Addr = ":0"
	srv := httptest.NewServer(app.newServer().Handler)
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
			name:       "HEAD returns headers without body",
			method:     http.MethodHead,
			path:       "/testorg/testrepo/style.css",
			wantURL:    "/testorg/testrepo/style.css",
			wantStatus: http.StatusOK,
			wantHeader: map[string]string{"Content-Type": "text/css; charset=utf-8"},
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
