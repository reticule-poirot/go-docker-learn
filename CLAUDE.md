# CLAUDE.md

Guidance for AI assistants (Claude Code or similar) working in this repo.

## What this is

A minimal Kubernetes-ready Go HTTP server (`hello.go`), built as a learning
project. Companion Helm chart lives in a sibling repo, `helm-k8s-learn`.

## Architecture

- Single-file app, stdlib `net/http` only (no router dependency — uses Go
  1.22+ method-pattern routing, e.g. `mux.HandleFunc("GET /health", ...)`).
- Structured logging via `log/slog`, JSON handler, written to stdout.
- Endpoints:
  - `GET /` — greets with pod name (`HOSTNAME`) and node name (`NODE_NAME`),
    both injected via the Kubernetes Downward API (see the Helm chart's
    `deployment.yaml`). Neither is set automatically by k8s — don't assume
    they'll be present outside a pod; both fall back gracefully.
  - `GET /health` — liveness probe. Keep this cheap: no dependency checks,
    no I/O. It answers "is the process alive", not "is it useful".
  - `GET /ready` — readiness probe. Backed by an `atomic.Bool` (`ready`),
    true only once the listener is actually bound, false again the instant
    shutdown begins. If you add downstream dependencies (DB, cache, etc.),
    this is where their health should be reflected — `/health` should stay
    dependency-free.
  - `GET /metrics` — hand-rolled Prometheus text format, no client library.
    If the metric surface grows past a few counters/gauges, switch to
    `client_golang`'s `promhttp` instead of extending this by hand.

## Known gotchas (already fixed once — don't reintroduce)

- **Bind before marking ready.** `main` calls `net.Listen` synchronously
  and only sets `ready.Store(true)` after that succeeds, inside the
  goroutine that then calls `srv.Serve(ln)`. Do NOT collapse this back into
  a bare `srv.ListenAndServe()` call with `ready.Store(true)` set before
  it — that reintroduces a race where the readiness probe can report healthy
  before the port is actually bound (this was the root cause of a
  "Readiness probe failed on first start" bug).
- **Graceful shutdown order matters**: `ready.Store(false)` happens
  *before* `srv.Shutdown(ctx)` is called, not after — this lets k8s stop
  routing new traffic here before the server stops accepting connections,
  minimizing dropped requests during a rolling update.
- Shutdown timeout (currently 20s) must stay comfortably under the pod's
  `terminationGracePeriodSeconds` (default 30s in the Helm chart) or k8s
  will SIGKILL mid-cleanup.

## Conventions

- Every `http.ResponseWriter.Write` return value is checked and logged via
  `slog.Error` on failure — don't silently discard it in new handlers.
- Runs as a non-root user in production (distroless `nonroot`, uid 65532) —
  don't add code that assumes root, writes to the container filesystem, or
  binds a privileged (<1024) port directly; the Helm chart maps external
  port 80 to container port 8000 for exactly this reason.
- Prefer stdlib over new dependencies unless there's a concrete need — this
  project intentionally avoids a router/framework dependency.

## Before committing

- `go build ./...` and `go vet ./...` should both pass cleanly.
- If `go vet` or a `gopls`-based `modernize` suggestion comes up (e.g.
  `fmt.Appendf` instead of `[]byte(fmt.Sprintf(...))`), it's generally safe
  to apply — these are mechanical, non-behavioral suggestions tied to the
  Go version in `go.mod`.
