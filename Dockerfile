FROM golang:1.25-bookworm AS builder

ENV GOPRIVATE=""
ENV GOPROXY=direct
ENV GOINSECURE="*"

# Set to 1 to disable TLS verification for git HTTPS calls during `go mod download`.
# Prefer supplying your Zscaler root CA instead, but this is a pragmatic escape hatch.
ARG INSECURE_SSL=0

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum* ./
RUN if [ "$INSECURE_SSL" = "1" ]; then \
        git config --global http.sslVerify false; \
        export GIT_SSL_NO_VERIFY=1; \
    fi \
    && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o prmate main.go

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    bash \
    curl \
    git \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install gh CLI via official Debian repository
RUN mkdir -p -m 755 /etc/apt/keyrings \
    && wget -nv -O /tmp/githubcli-archive-keyring.gpg https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    && cat /tmp/githubcli-archive-keyring.gpg | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update \
    && apt-get install gh -y \
    && rm -rf /var/lib/apt/lists/*

# Verify gh installation
RUN gh --version

# Install Copilot CLI
RUN wget --no-check-certificate -qO- https://gh.io/copilot-install | bash && \
    copilot --version

WORKDIR /app
COPY --from=builder /app/prmate .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./prmate"]
