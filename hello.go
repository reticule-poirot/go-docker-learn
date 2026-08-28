package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

var port int

// ready is flipped to true once the server is up, and back to false
// as soon as shutdown begins, so k8s stops routing traffic immediately.
var ready atomic.Bool

// requestCount is a minimal in-process counter exposed via /metrics.
var requestCount atomic.Uint64

var startTime = time.Now()

func withMetrics(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		next(w, r)
	}
}

// health is the liveness probe: process is up and not deadlocked.
// Deliberately cheap — no dependency checks.
func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		slog.Error("health: write failed", "error", err)
	}
}

// readyHandler is the readiness probe: can this pod serve traffic.
// Reflects real state via the `ready` flag (false during startup/shutdown,
// and would reflect downstream dependency checks if/when added).
func readyHandler(w http.ResponseWriter, r *http.Request) {
	if !ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("not ready")); err != nil {
			slog.Error("ready: write failed", "error", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ready")); err != nil {
		slog.Error("ready: write failed", "error", err)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	hostname := os.Getenv("HOSTNAME")
	// NODE_NAME is not set by Kubernetes automatically like HOSTNAME (pod name) is —
	// it must be injected via the Downward API in the pod spec (fieldRef: spec.nodeName).
	// Falls back to "unknown" when running outside k8s (e.g. local/docker run).
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "unknown"
	}
	slog.Info("handling request", "hostname", hostname, "node", nodeName, "path", r.URL.Path)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(fmt.Sprintf("Hello, I'm %s, from node %s!\n", hostname, nodeName))); err != nil {
		slog.Error("home: write failed", "error", err)
	}
}

// metrics is a minimal hand-rolled Prometheus-text-format endpoint.
// No external dependency; swap in a real client library if the
// metric surface grows beyond a handful of counters/gauges.
func metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	uptime := time.Since(startTime).Seconds()
	readyVal := 0
	if ready.Load() {
		readyVal = 1
	}
	fmt.Fprintf(w, "# HELP app_requests_total Total number of requests handled.\n")
	fmt.Fprintf(w, "# TYPE app_requests_total counter\n")
	fmt.Fprintf(w, "app_requests_total %d\n", requestCount.Load())
	fmt.Fprintf(w, "# HELP app_uptime_seconds Time since process start, in seconds.\n")
	fmt.Fprintf(w, "# TYPE app_uptime_seconds gauge\n")
	fmt.Fprintf(w, "app_uptime_seconds %f\n", uptime)
	fmt.Fprintf(w, "# HELP app_ready Whether the app is currently marked ready (1) or not (0).\n")
	fmt.Fprintf(w, "# TYPE app_ready gauge\n")
	fmt.Fprintf(w, "app_ready %d\n", readyVal)
}

func main() {
	flag.IntVar(&port, "port", 8000, "an int")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting server", "port", port)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", withMetrics(home))
	mux.HandleFunc("GET /health", withMetrics(health))
	mux.HandleFunc("GET /ready", withMetrics(readyHandler))
	mux.HandleFunc("GET /metrics", metrics)

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Serve in the background, mark ready once listening succeeds.
	serveErrCh := make(chan error, 1)
	go func() {
		ready.Store(true)
		slog.Info("marked ready")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
			return
		}
		serveErrCh <- nil
	}()

	// Wait for SIGTERM/SIGINT (k8s sends SIGTERM on pod termination).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serveErrCh:
		if err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	case sig := <-sigCh:
		slog.Info("received shutdown signal", "signal", sig.String())

		// Flip readiness false immediately so k8s stops routing new
		// traffic here well before the process actually exits.
		ready.Store(false)

		// Give in-flight requests time to finish. Keep this safely
		// under the pod's terminationGracePeriodSeconds (default 30s).
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		slog.Info("shutdown complete")
	}
}
