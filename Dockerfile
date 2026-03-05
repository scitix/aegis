# ── Stage 1: download kubectl ────────────────────────────────────────────────
FROM curlimages/curl:8.8.0 AS kubectl
ARG KUBECTL_VERSION=v1.29.8
RUN curl -L -o /tmp/kubectl \
      "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    chmod +x /tmp/kubectl

# ── Stage 2: build Go binaries ──────────────────────────────────────────────
FROM golang:1.24.2 AS builder
WORKDIR /aegis
ADD . /aegis/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aegis            cmd/aegis/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aegis-selfhealing cmd/aegis-selfhealing/main.go

# ── Stage 3: minimal runtime image ─────────────────────────────────────────
FROM ubuntu:22.04
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /aegis
COPY --from=builder /aegis/aegis            /aegis/
COPY --from=builder /aegis/aegis-selfhealing /aegis/
COPY --from=kubectl /tmp/kubectl            /usr/local/bin/kubectl

# collector scripts (used by diagnosis)
COPY manifests/collector/ /collector/
# selfhealing job manifests
COPY manifests/job/ /selfhealing/job/
