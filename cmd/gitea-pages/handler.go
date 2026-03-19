package main

import (
	"fmt"
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

	// Serve index.html for directory requests (root or trailing slash).
	if rawPath == "" || strings.HasSuffix(rawPath, "/") {
		filePath := rawPath + "index.html"

		content, err := app.getFile(owner, repo, filePath)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		writeContent(w, filePath, content)
		return
	}

	// Try exact file.
	content, err := app.getFile(owner, repo, rawPath)
	if err == nil {
		writeContent(w, rawPath, content)
		return
	}

	// Check if directory with index.html exists → redirect to trailing slash.
	if _, err := app.getFile(owner, repo, rawPath+"/index.html"); err == nil {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func writeContent(w http.ResponseWriter, filePath string, content []byte) {
	if ct := mime.TypeByExtension(path.Ext(filePath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	if _, err := w.Write(content); err != nil { //nolint:gosec // G705: serving static files from Gitea repos is intentional
		slog.Error("Failed to write response", "error", err)
	}
}

func (app *App) getFile(owner, repo, filePath string) ([]byte, error) {
	content, _, err := app.Client.GetFile(owner, repo, app.PagesBranch, filePath, true)
	if err != nil {
		return nil, fmt.Errorf("fetching %s/%s/%s: %w", owner, repo, filePath, err)
	}

	return content, nil
}

// handleRepoRedirect redirects /{owner}/{repo} to /{owner}/{repo}/.
func handleRepoRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
}
