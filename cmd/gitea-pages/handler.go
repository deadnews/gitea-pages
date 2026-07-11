package main

import (
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
)

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handlePages serves static files from Gitea repositories.
func (app *App) handlePages(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	rawPath := r.PathValue("path")

	// Directory requests (root or trailing slash) serve index.html.
	isDir := rawPath == "" || strings.HasSuffix(rawPath, "/")
	filePath := rawPath
	if isDir {
		filePath += "index.html"
	}
	if content, ok := app.getFile(owner, repo, filePath); ok {
		writeContent(w, filePath, content)
		return
	}

	// Directory without trailing slash → redirect if its index.html exists.
	if !isDir {
		if _, ok := app.getFile(owner, repo, rawPath+"/index.html"); ok {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently) //nolint:gosec // G710
			return
		}
	}

	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func writeContent(w http.ResponseWriter, filePath string, content []byte) {
	if ct := mime.TypeByExtension(path.Ext(filePath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	if _, err := w.Write(content); err != nil { //nolint:gosec // G705
		slog.Error("Failed to write response", "error", err)
	}
}

func (app *App) getFile(owner, repo, filePath string) ([]byte, bool) {
	content, resp, err := app.client.GetFile(owner, repo, app.config.PagesBranch, filePath, true)
	if err != nil {
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			slog.Warn("Failed to fetch from Gitea", "owner", owner, "repo", repo, "path", filePath, "error", err)
		}
		return nil, false
	}
	return content, true
}

// handleRepoRedirect redirects /{owner}/{repo} to /{owner}/{repo}/.
func handleRepoRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently) //nolint:gosec // G710
}
