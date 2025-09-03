FROM --platform=$BUILDPLATFORM golang:1.23 AS builder
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV GOMODCACHE=/go/pkg/mod
ENV GOFLAGS="-trimpath"

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -v -ldflags="-s -w" ./cmd/kompoxops

FROM --platform=$TARGETPLATFORM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget curl git gnupg lsb-release rsync vim \
    && wget -q -O - https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor -o /etc/apt/trusted.gpg.d/microsoft.gpg \
    && echo "deb [arch=amd64,arm64 signed-by=/etc/apt/trusted.gpg.d/microsoft.gpg] https://packages.microsoft.com/repos/azure-cli/ noble main" > /etc/apt/sources.list.d/azure-cli.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends azure-cli \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://aka.ms/install-azd.sh | bash && azd version

RUN set -eux; \
    case "$(uname -m)" in \
    x86_64|amd64) AZCOPY_URL="https://aka.ms/downloadazcopy-v10-linux" ;; \
    aarch64|arm64) AZCOPY_URL="https://aka.ms/downloadazcopy-v10-linux-arm64" ;; \
    *) echo "Unsupported arch: $(uname -m)"; exit 1 ;; \
    esac; \
    curl -fsSL "$AZCOPY_URL" -o /tmp/azcopy.tgz; \
    tar -xzf /tmp/azcopy.tgz -C /tmp; \
    install -m 0755 $(find /tmp -type f -name azcopy | head -n1) /usr/local/bin/azcopy; \
    azcopy --version; \
    rm -rf /tmp/azcopy*

COPY --from=builder /src/kompoxops /usr/local/bin/kompoxops

# rsyncd.conf for exporting /vol as module "vol"
RUN mkdir -p /vol && \
    printf '%s\n' \
      'uid = root' \
      'gid = root' \
      'use chroot = no' \
      'max connections = 8' \
      'pid file = /var/run/rsyncd.pid' \
      'log file = /dev/stdout' \
      '[vol]' \
      '  path = /vol' \
      '  read only = false' \
      > /etc/rsyncd.conf

EXPOSE 873

# Default entrypoint: kompoxops CLI
ENTRYPOINT ["/usr/local/bin/kompoxops"]

# Optional: rsyncd as a service (uncomment to use as rsyncd container)
# ENTRYPOINT ["rsync", "--daemon", "--no-detach"]
