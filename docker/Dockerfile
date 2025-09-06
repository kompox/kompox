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
    ca-certificates wget curl git gnupg lsb-release rsync vim sudo openssh-server tini \
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

RUN useradd -m -s /bin/bash kompox \
    && echo 'kompox ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/010-kompox \
    && install -d -m 755 /var/run/sshd \
    && printf '%s\n' \
    'PermitRootLogin yes' \
    'PubkeyAuthentication yes' \
    'Ciphers chacha20-poly1305@openssh.com,aes128-ctr' \
    'KexAlgorithms curve25519-sha256' \
    'Match User root,kompox' \
    'AuthorizedKeysFile /etc/ssh/authorized_keys .ssh/authorized_keys .ssh/authorized_keys2' \
    > /etc/ssh/sshd_config.d/10-remote-ssh.conf

EXPOSE 22

COPY --from=builder /src/kompoxops /usr/local/bin/kompoxops

COPY docker/entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
