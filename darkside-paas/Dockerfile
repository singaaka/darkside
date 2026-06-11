# syntax=docker/dockerfile:1.7

# ---- Stage 1: build the frontend ----
FROM node:22-alpine AS frontend-builder
RUN corepack enable
WORKDIR /work
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm run build

# ---- Stage 2: build the Go binary ----
FROM golang:1.25-alpine AS backend-builder
WORKDIR /work
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /work/dist ./internal/frontend/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/darkside-paas ./cmd/darkside-paas

# ---- Stage 3: runtime ----
FROM alpine:3.20
ARG TARGETARCH=amd64
ARG NOMAD_VERSION=1.9.0
RUN apk add --no-cache ca-certificates docker-cli git tzdata wget unzip \
 && wget -q "https://releases.hashicorp.com/nomad/${NOMAD_VERSION}/nomad_${NOMAD_VERSION}_linux_${TARGETARCH}.zip" -O /tmp/nomad.zip \
 && unzip -d /usr/local/bin /tmp/nomad.zip \
 && rm /tmp/nomad.zip \
 && apk del wget unzip
COPY --from=backend-builder /out/darkside-paas /usr/local/bin/darkside-paas
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/darkside-paas"]
