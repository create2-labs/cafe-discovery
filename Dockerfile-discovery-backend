##############################
# Stage 1 — Build (Go + OQS)
##############################
FROM oleglod/cafe-crypto-backend:build-oqs AS builder

ARG TARGETARCH
ARG APP_VERSION
ENV DEBIAN_FRONTEND=noninteractive

# ---- Install Go (BUILD ONLY) ----
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Go manually (multi-arch)
ENV GO_VERSION=1.26.4
RUN set -eux; \
    case "$TARGETARCH" in \
    amd64)  GO_ARCH=amd64 ;; \
    arm64)  GO_ARCH=arm64 ;; \
    *) echo "unsupported arch $TARGETARCH"; exit 1 ;; \
    esac; \
    wget -q https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz; \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-${GO_ARCH}.tar.gz; \
    rm go${GO_VERSION}.linux-${GO_ARCH}.tar.gz

ENV PATH=/usr/local/go/bin:$PATH
ENV CGO_ENABLED=1

# ---- CGO + OQS flags ----
ENV CGO_CFLAGS="-I/opt/liboqs/include"
ENV CGO_LDFLAGS="-L/opt/liboqs/lib -loqs -lssl -lcrypto -Wl,-rpath,/opt/liboqs/lib"
ENV PKG_CONFIG_PATH=/opt/liboqs/lib/pkgconfig

WORKDIR /app

# ---- Go deps ----
COPY go.mod go.sum ./
RUN go mod download

# ---- Sources ----
COPY . .

# ---- Build ----
RUN RESOLVED_VERSION="${APP_VERSION}" && \
    if [ -z "$RESOLVED_VERSION" ]; then \
      if command -v git >/dev/null 2>&1 && [ -d .git ]; then \
        BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null | sed 's/[^a-zA-Z0-9]/-/g' || echo "dev") && \
        SHORT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "") && \
        RESOLVED_VERSION="${BRANCH}-${SHORT_SHA:-unknown}"; \
      else \
        RESOLVED_VERSION="dev"; \
      fi; \
    fi && \
    go build \
      -ldflags "-X cafe-discovery/internal/version.version=${RESOLVED_VERSION}" \
      -o server ./cmd/server/main.go


##############################
# Stage CI — Lint / Test / Vuln (use: docker build --target ci)
##############################
FROM builder AS ci

WORKDIR /app

# Cache-bust: force reinstall of tools when Go version changes (must match GO_VERSION in builder)
ENV GO_TOOLING_VERSION=1.26.4
ENV PATH=/root/go/bin:/usr/local/go/bin:/usr/local/bin:$PATH

RUN go install golang.org/x/vuln/cmd/govulncheck@latest \
    && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0

CMD ["sh", "-c", "go mod download && golangci-lint run ./... && go test ./... && /root/go/bin/govulncheck ./..."]


##############################
# Stage 2 — Runtime (default stage: last = image used without --target)
##############################
FROM oleglod/cafe-crypto-backend:runtime-oqs AS runtime

# Build argument for version (injected by GitHub Actions)
ARG APP_VERSION
ARG TARGETARCH

# Set version label (OCI standard)
LABEL org.opencontainers.image.version="${APP_VERSION}"

WORKDIR /app

# Copy application binary
COPY --from=builder /app/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
