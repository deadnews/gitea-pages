package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	handleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandlePages(t *testing.T) {
	app := newFakeGitea(t).newApp()
	server := app.newServer()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
		wantHeader map[string]string
	}{
		{
			name:       "serves index.html for repo root",
			path:       "/testorg/testrepo/",
			wantStatus: http.StatusOK,
			wantBody:   "<html>index</html>",
			wantHeader: map[string]string{"Content-Type": "text/html; charset=utf-8"},
		},
		{
			name:       "serves specific file",
			path:       "/testorg/testrepo/style.css",
			wantStatus: http.StatusOK,
			wantBody:   "body {}",
			wantHeader: map[string]string{
				"Content-Type":   "text/css; charset=utf-8",
				"Content-Length": "7",
			},
		},
		{
			name:       "serves nested file",
			path:       "/testorg/testrepo/assets/app.js",
			wantStatus: http.StatusOK,
			wantBody:   "console.log('hello')",
			wantHeader: map[string]string{"Content-Type": "text/javascript; charset=utf-8"},
		},
		{
			name:       "serves directory index.html fallback",
			path:       "/testorg/testrepo/subdir/",
			wantStatus: http.StatusOK,
			wantBody:   "<html>subdir</html>",
		},
		{
			name:       "redirects directory without trailing slash",
			path:       "/testorg/testrepo/subdir",
			wantStatus: http.StatusMovedPermanently,
			wantHeader: map[string]string{"Location": "/testorg/testrepo/subdir/"},
		},
		{
			name:       "redirects directory whose name needs escaping",
			path:       "/testorg/testrepo/a%3Fb",
			wantStatus: http.StatusMovedPermanently,
			wantHeader: map[string]string{"Location": "/testorg/testrepo/a%3Fb/"},
		},
		{
			name:       "serves file without recognized extension",
			path:       "/testorg/testrepo/data",
			wantStatus: http.StatusOK,
			wantBody:   "raw data",
		},
		{
			name:       "returns 404 for missing file",
			path:       "/testorg/testrepo/missing.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "returns 404 for missing directory",
			path:       "/testorg/testrepo/nonexistent/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "returns 404 for missing repo",
			path:       "/testorg/norepo/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "serves file whose name needs escaping",
			path:       "/testorg/testrepo/a%20b.html",
			wantStatus: http.StatusOK,
			wantBody:   "<html>spaced</html>",
		},
		{
			name:       "rejects encoded parent traversal",
			path:       "/testorg/testrepo/%2e%2e/%2e%2e/index.html",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "keeps branch pin against encoded query char",
			path:       "/testorg/testrepo/default-only.txt%3Fx",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "keeps branch pin against encoded fragment char",
			path:       "/testorg/testrepo/default-only.txt%23x",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)

			rec := httptest.NewRecorder()
			server.Handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, rec.Body.String())
			}
			for k, v := range tt.wantHeader {
				assert.Equal(t, v, rec.Header().Get(k))
			}
		})
	}
}

// Directory requests that miss must 404 after a single fetch: probing
// "{path}//index.html" could cause spurious redirects on a normalizing backend.
func TestHandlePagesDirectoryMissSingleFetch(t *testing.T) {
	fg := newFakeGitea(t)
	server := fg.newApp().newServer()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/testorg/testrepo/nonexistent/", http.NoBody)

	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, int32(1), fg.fileRequests.Load())
}

func TestHandlePagesDotSegments(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "parent as owner", path: "/%2e%2e/repos/testrepo/index.html"},
		{name: "current as owner", path: "/%2e/testorg/testrepo/index.html"},
		{name: "parent as repo", path: "/testorg/%2e%2e/testrepo/index.html"},
		{name: "current in file path", path: "/testorg/testrepo/%2e/index.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fg := newFakeGitea(t)
			server := fg.newApp().newServer()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			server.Handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Zero(t, fg.fileRequests.Load())
		})
	}
}

func TestHandlePagesUpstreamFailure(t *testing.T) {
	fg := newFakeGitea(t)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	tests := []struct {
		name   string
		server string
		token  string
	}{
		{
			name:   "returns 404 when Gitea rejects the token",
			server: fg.server.URL,
			token:  "wrong-token",
		},
		{
			name:   "returns 404 when Gitea is unreachable",
			server: closed.URL,
			token:  fakeToken,
		},
		{
			name:   "returns 404 for malformed server URL",
			server: "://invalid-url",
			token:  fakeToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newApp(&config{GiteaServer: tt.server, GiteaToken: tt.token, PagesBranch: "gh-pages"})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/testorg/testrepo/", http.NoBody)
			rec := httptest.NewRecorder()
			app.newServer().Handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestHandleRepoRedirect(t *testing.T) {
	app := newFakeGitea(t).newApp()
	server := app.newServer()

	tests := []struct {
		name         string
		path         string
		wantLocation string
	}{
		{
			name:         "adds trailing slash",
			path:         "/testorg/testrepo",
			wantLocation: "/testorg/testrepo/",
		},
		{
			name:         "keeps encoded slash from targeting another host",
			path:         "/%2Fevil.example/x",
			wantLocation: "/%2Fevil.example/x/",
		},
		{
			name:         "keeps encoded backslash from targeting another host",
			path:         "/%5Cevil.example/x",
			wantLocation: "/%5Cevil.example/x/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)

			rec := httptest.NewRecorder()
			server.Handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusMovedPermanently, rec.Code)
			assert.Equal(t, tt.wantLocation, rec.Header().Get("Location"))
		})
	}
}
