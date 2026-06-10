# syntax=docker/dockerfile:1.7

# ---- Stage 1: generate protobuf + connect code from .proto ----
FROM bufbuild/buf:1.45.0 AS proto-gen
WORKDIR /work
COPY buf.yaml buf.gen.yaml ./
COPY proto ./proto
RUN buf generate

# ---- Stage 2: build the frontend ----
FROM node:22-alpine AS frontend-builder
WORKDIR /work
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
COPY --from=proto-gen /work/frontend/src/gen ./src/gen
RUN npm run build

# ---- Stage 3: build the Go binary ----
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache git
WORKDIR /work
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=proto-gen /work/gen ./gen
COPY --from=frontend-builder /work/dist ./internal/frontend/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/darkside ./cmd/darkside

# ---- Stage 4: runtime ----
FROM alpine:3.20
ARG TARGETARCH=amd64
ARG NOMAD_VERSION=1.9.0
RUN apk add --no-cache ca-certificates docker-cli git tzdata wget unzip \
 && wget -q "https://releases.hashicorp.com/nomad/${NOMAD_VERSION}/nomad_${NOMAD_VERSION}_linux_${TARGETARCH}.zip" -O /tmp/nomad.zip \
 && unzip -d /usr/local/bin /tmp/nomad.zip \
 && rm /tmp/nomad.zip \
 && apk del wget unzip
WORKDIR /
COPY --from=backend-builder /out/darkside /usr/local/bin/darkside
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/darkside"]
