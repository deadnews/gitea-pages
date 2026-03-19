package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureLog redirects slog to a buffer for the test duration.
func captureLog(t *testing.T, opts *slog.HandlerOptions) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, opts)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return &buf
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		tests := []struct {
			name   string
			status int
		}{
			{"200", http.StatusOK},
			{"201", http.StatusCreated},
			{"400", http.StatusBadRequest},
			{"404", http.StatusNotFound},
			{"500", http.StatusInternalServerError},
			{"503", http.StatusServiceUnavailable},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

				rw.WriteHeader(tt.status)

				assert.Equal(t, tt.status, rw.status)
				assert.Equal(t, tt.status, rec.Code)
			})
		}
	})

	t.Run("unwrap returns underlying writer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

		assert.Equal(t, rec, rw.Unwrap())
	})
}

func TestLoggerMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		handlerStatus  int
		expectLogged   bool
		expectLogLevel slog.Level
	}{
		{
			name:          "health endpoint not logged",
			path:          "/health",
			handlerStatus: http.StatusOK,
			expectLogged:  false,
		},
		{
			name:          "2xx not logged",
			path:          "/query",
			handlerStatus: http.StatusOK,
			expectLogged:  false,
		},
		{
			name:          "3xx not logged",
			path:          "/redirect",
			handlerStatus: http.StatusMovedPermanently,
			expectLogged:  false,
		},
		{
			name:           "4xx logged at warn",
			path:           "/query",
			handlerStatus:  http.StatusBadRequest,
			expectLogged:   true,
			expectLogLevel: slog.LevelWarn,
		},
		{
			name:           "404 logged at warn",
			path:           "/notfound",
			handlerStatus:  http.StatusNotFound,
			expectLogged:   true,
			expectLogLevel: slog.LevelWarn,
		},
		{
			name:           "5xx logged at error",
			path:           "/query",
			handlerStatus:  http.StatusInternalServerError,
			expectLogged:   true,
			expectLogLevel: slog.LevelError,
		},
		{
			name:           "503 logged at error",
			path:           "/unavailable",
			handlerStatus:  http.StatusServiceUnavailable,
			expectLogged:   true,
			expectLogLevel: slog.LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t, &slog.HandlerOptions{Level: slog.LevelDebug})

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.handlerStatus)
			})

			wrapped := Logger(handler)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)

			logOutput := buf.String()
			if tt.expectLogged {
				assert.Contains(t, logOutput, "request")
				assert.Contains(t, logOutput, tt.path)
				assert.Contains(t, logOutput, "method=GET")
				assert.Contains(t, logOutput, fmt.Sprintf("status=%d", tt.handlerStatus))
				assert.Contains(t, logOutput, fmt.Sprintf("level=%s", tt.expectLogLevel))
			} else {
				assert.Empty(t, logOutput)
			}
		})
	}
}

func TestLoggerMiddlewareRequestDetails(t *testing.T) {
	buf := captureLog(t, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	wrapped := Logger(handler)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/query?param=value", http.NoBody)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "method=POST")
	assert.Contains(t, logOutput, "/query")
	assert.Contains(t, logOutput, "TestAgent/1.0")
	assert.Contains(t, logOutput, "status=404")
	assert.Contains(t, logOutput, "duration=")
}

func TestRecovererMiddleware(t *testing.T) {
	t.Run("recovers from panic with string", func(t *testing.T) {
		buf := captureLog(t, nil)

		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("test panic")
		})

		wrapped := Recoverer(handler)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", http.NoBody)
		rec := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			wrapped.ServeHTTP(rec, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, buf.String(), "panic recovered")
		assert.Contains(t, buf.String(), "test panic")
	})

	t.Run("recovers from panic with error", func(t *testing.T) {
		captureLog(t, nil)

		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic(io.ErrUnexpectedEOF)
		})

		wrapped := Recoverer(handler)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", http.NoBody)
		rec := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			wrapped.ServeHTTP(rec, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("passes through without panic", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		})

		wrapped := Recoverer(handler)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/normal", http.NoBody)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "success", rec.Body.String())
	})

	t.Run("logs path", func(t *testing.T) {
		buf := captureLog(t, nil)

		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("test")
		})

		wrapped := Recoverer(handler)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test/path?query=value", http.NoBody)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Contains(t, buf.String(), "/test/path")
	})
}

func TestMiddlewareChain(t *testing.T) {
	t.Run("logger and recoverer chain success not logged", func(t *testing.T) {
		buf := captureLog(t, nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := Logger(Recoverer(handler))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, buf.String())
	})

	t.Run("panic in chain is recovered and logged", func(t *testing.T) {
		buf := captureLog(t, nil)

		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("chain panic")
		})

		wrapped := Logger(Recoverer(handler))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody)
		rec := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			wrapped.ServeHTTP(rec, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, buf.String(), "panic recovered")
		assert.Contains(t, buf.String(), "request")
		assert.Contains(t, buf.String(), "status=500")
	})
}
