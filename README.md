# go-docker-learn

A small Kubernetes-ready Go HTTP server, built while learning Go and Docker
basics. Uses only the Go standard library (`net/http`, `log/slog`) — no
router dependency.

Companion Helm chart for deploying this app: [`helm-k8s-learn`](https://github.com/reticule-poirot/helm-k8s-learn).

## Endpoints

| Route          | Purpose                                                            |
|----------------|---------------------------------------------------------------------|
| `GET /`        | Greets with the pod name and node name (via k8s Downward API)       |
| `GET /health`  | Liveness probe — cheap, no dependency checks                        |
| `GET /ready`   | Readiness probe — reflects whether the server is actually serving   |
| `GET /metrics` | Minimal Prometheus-format metrics (request count, uptime, readiness) |

## Running locally

With Docker Compose:

```bash
docker compose up --build
```

The app will be available at http://localhost:8000.

Without Docker:

```bash
go run . -port=8000
```

`NODE_NAME` and `HOSTNAME` are only populated automatically inside a
Kubernetes pod (see the Helm chart's Downward API wiring) — running
locally, `/` will show `node: unknown`.

## Building the container image

```bash
docker build -t myapp:0.0.1 .
```

Tag this to match `appVersion` in the Helm chart's `Chart.yaml` — that's
what the chart uses by default when `image.tag` is left unset.

If your target platform differs from your dev machine (e.g. building on
Apple Silicon for an amd64 cluster):

```bash
docker build --platform=linux/amd64 -t myapp:0.0.1 .
```

The final image is based on `gcr.io/distroless/static-debian12:nonroot` —
no shell, no package manager, runs as a non-root user (uid 65532).

### Loading into a local cluster

If you're running a local `kind` cluster (as opposed to a remote registry),
load the image directly rather than pushing it anywhere:

```bash
kind load docker-image myapp:0.0.1 --name desktop
```

(`--name desktop` targets a kind cluster named `desktop` — adjust to match
your actual cluster name, e.g. from `kind get clusters`.)

If you're instead using Docker Desktop's built-in Kubernetes (not `kind`),
no load step is needed at all — it shares the same Docker daemon you just
built the image with, so `myapp:0.0.1` is already visible to the cluster.

Then reference `myapp` / `0.0.1` as `image.repository` / `image.tag` when
installing the Helm chart — no registry needed.

## Design notes

- Graceful shutdown on `SIGTERM`/`SIGINT`: stops accepting new connections,
  flips readiness false immediately, lets in-flight requests finish.
- The listener is bound explicitly (`net.Listen`) before the readiness flag
  is ever set true, to avoid a startup race where the probe could report
  healthy before the port was actually listening.
- See `CLAUDE.md` for the fuller set of gotchas and conventions if you're
  working on this with an AI coding assistant.

## References

* [Docker's Go guide](https://docs.docker.com/language/golang/)
