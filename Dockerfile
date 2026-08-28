# syntax=docker/dockerfile:1

################################################################################
# Build stage
ARG GO_VERSION=1.23.4
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src

# Download dependencies as a separate, cached layer.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    GOFLAGS=-mod=readonly go mod download -x

ARG TARGETARCH

# Build a fully static, stripped, reproducible binary.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-w -s" -o /bin/server .

################################################################################
# Final stage: distroless static image, no shell, no package manager.
# "nonroot" variant already runs as uid/gid 65532 and includes CA certs +
# tzdata, so no extra apk/apt step is needed here.
FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=build /bin/server /bin/server

EXPOSE 8000

USER nonroot:nonroot

ENTRYPOINT ["/bin/server"]