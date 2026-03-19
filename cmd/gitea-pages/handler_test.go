package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	handleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandlePages(t *testing.T) {
	fg := newFakeGitea(t)

	app := &App{Client: fg.client, PagesBranch: "gh-pages"}
	cfg := &Config{Addr: ":8000"}
	server := setupServer(cfg, app)

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
			wantHeader: map[string]string{"Content-Type": "text/css; charset=utf-8"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			require.NoError(t, err)

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

func TestHandleRepoRedirect(t *testing.T) {
	fg := newFakeGitea(t)

	app := &App{Client: fg.client, PagesBranch: "gh-pages"}
	cfg := &Config{Addr: ":8000"}
	server := setupServer(cfg, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/testorg/testrepo", http.NoBody)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMovedPermanently, rec.Code)
	assert.Equal(t, "/testorg/testrepo/", rec.Header().Get("Location"))
}
