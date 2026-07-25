package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureLog redirects slog to a buffer for the test duration.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return &buf
}

func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, rw.status)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, rec, rw.Unwrap())
}

func TestLogger(t *testing.T) {
	tests := []struct {
		name   string
		status int
		level  slog.Level
		logged bool
	}{
		{name: "2xx not logged", status: http.StatusOK},
		{name: "3xx not logged", status: http.StatusMovedPermanently},
		{name: "4xx logged at warn", status: http.StatusNotFound, level: slog.LevelWarn, logged: true},
		{name: "5xx logged at error", status: http.StatusInternalServerError, level: slog.LevelError, logged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/path?param=value", http.NoBody)
			req.Header.Set("User-Agent", "TestAgent/1.0")
			Logger(handler).ServeHTTP(httptest.NewRecorder(), req)

			out := buf.String()
			if !tt.logged {
				assert.Empty(t, out)
				return
			}

			assert.Contains(t, out, fmt.Sprintf("level=%s msg=Request", tt.level))
			assert.Contains(t, out, "method=POST")
			assert.Contains(t, out, "/path?param=value")
			assert.Contains(t, out, "user_agent=TestAgent/1.0")
			assert.Contains(t, out, fmt.Sprintf("status=%d", tt.status))
			assert.Contains(t, out, "duration=")
		})
	}
}

func TestRecoverer(t *testing.T) {
	buf := captureLog(t)

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/path", http.NoBody)
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		Logger(Recoverer(handler)).ServeHTTP(rec, req)
	})

	out := buf.String()
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, out, "Panic recovered")
	assert.Contains(t, out, "test panic")
	assert.Contains(t, out, "url=/path")
	assert.Contains(t, out, "status=500")
}
