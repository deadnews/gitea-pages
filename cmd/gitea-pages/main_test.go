package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// freeAddr reserves and releases a port, returning an address free to bind.
func freeAddr(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())

	return listener.Addr().String()
}

// waitListening blocks until addr accepts connections,
// which means run already installed its signal handler.
func waitListening(t *testing.T, addr string) {
	t.Helper()

	var d net.Dialer
	require.Eventually(t, func() bool {
		conn, err := d.DialContext(t.Context(), "tcp", addr)
		if err != nil {
			return false
		}
		_ = conn.Close()

		return true
	}, 10*time.Second, 10*time.Millisecond)
}

// holdTerm keeps SIGTERM handled for the whole test,
// so a signal that misses run fails the test instead of killing the binary.
func holdTerm(t *testing.T) {
	t.Helper()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(sig) })
}

func TestRunConfigError(t *testing.T) {
	t.Setenv("GITEA_PAGES_SERVER", "")
	t.Setenv("GITEA_PAGES_TOKEN", "")

	require.ErrorContains(t, run(), "load config")
}

func TestRunClientError(t *testing.T) {
	t.Setenv("GITEA_PAGES_SERVER", "://invalid-url")
	t.Setenv("GITEA_PAGES_TOKEN", "test-token")

	require.ErrorContains(t, run(), "init application")
}

func TestRunListenError(t *testing.T) {
	fg := newFakeGitea(t)
	t.Setenv("GITEA_PAGES_SERVER", fg.server.URL)
	t.Setenv("GITEA_PAGES_TOKEN", "test-token")
	t.Setenv("GITEA_PAGES_ADDR", "127.0.0.1:99999")

	require.ErrorContains(t, run(), "server error")
}

func TestRunGracefulShutdown(t *testing.T) {
	fg := newFakeGitea(t)
	addr := freeAddr(t)
	t.Setenv("GITEA_PAGES_SERVER", fg.server.URL)
	t.Setenv("GITEA_PAGES_TOKEN", "test-token")
	t.Setenv("GITEA_PAGES_ADDR", addr)

	holdTerm(t)

	done := make(chan error, 1)
	go func() { done <- run() }()

	waitListening(t, addr)

	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, proc.Signal(syscall.SIGTERM))

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return after SIGTERM")
	}
}
