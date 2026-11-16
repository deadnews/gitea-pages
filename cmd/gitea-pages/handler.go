package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
)

var errNotFound = errors.New("not found")

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
	if content, ok := app.getFile(r.Context(), owner, repo, filePath); ok {
		writeContent(w, filePath, content)
		return
	}

	// Directory without trailing slash → redirect if its index.html exists.
	if !isDir {
		if _, ok := app.getFile(r.Context(), owner, repo, rawPath+"/index.html"); ok {
			dir := r.URL.EscapedPath() + "/"
			http.Redirect(w, r, dir, http.StatusMovedPermanently) //nolint:gosec // G710
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

func (app *App) getFile(ctx context.Context, owner, repo, filePath string) ([]byte, bool) {
	content, err := app.fetchFile(ctx, owner, repo, filePath)
	if err != nil {
		if !errors.Is(err, errNotFound) {
			slog.Warn("Failed to fetch from Gitea",
				"owner", owner, "repo", repo, "path", filePath, "error", err)
		}
		return nil, false
	}
	return content, true
}

func (app *App) fetchFile(ctx context.Context, owner, repo, filePath string) ([]byte, error) {
	escaped, ok := escapeFilePath(filePath)
	if !ok || isDotSegment(owner) || isDotSegment(repo) {
		return nil, errNotFound
	}

	// The media endpoint resolves LFS pointers, the raw one returns them as is.
	fileURL := fmt.Sprintf("%s/repos/%s/%s/media/%s?ref=%s", app.apiURL,
		url.PathEscape(owner), url.PathEscape(repo), escaped,
		url.QueryEscape(app.config.PagesBranch))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, http.NoBody) //nolint:gosec // G704
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+app.config.GiteaToken)

	resp, err := app.client.Do(req) //nolint:gosec // G704
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return content, nil
}

// escapeFilePath encodes each segment so the path cannot alter the upstream
// request URL, and reports false for a dot segment.
func escapeFilePath(filePath string) (string, bool) {
	segments := strings.Split(filePath, "/")
	for i, segment := range segments {
		if isDotSegment(segment) {
			return "", false
		}
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/"), true
}

// isDotSegment reports a segment the upstream would resolve to another path.
func isDotSegment(segment string) bool {
	return segment == "." || segment == ".."
}

// handleRepoRedirect redirects /{owner}/{repo} to /{owner}/{repo}/.
func handleRepoRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, r.URL.EscapedPath()+"/", http.StatusMovedPermanently) //nolint:gosec // G710
}
