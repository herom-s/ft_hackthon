# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/ft_hackthon-api   ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/ft_hackthon-worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/ft_hackthon-cli    ./cmd/ft_hackthon && \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/healthcheck       ./cmd/healthcheck

# API runtime stage
FROM gcr.io/distroless/static-debian12:nonroot AS api
WORKDIR /app
COPY --from=builder /app/bin/ft_hackthon-api /app/ft_hackthon-api
COPY --from=builder /app/bin/healthcheck /app/healthcheck
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/app/ft_hackthon-api"]

# Worker runtime stage (needs git + docker-cli)
FROM alpine:latest AS worker
WORKDIR /app
RUN apk add --no-cache ca-certificates git docker-cli
COPY --from=builder /app/bin/ft_hackthon-worker /app/ft_hackthon-worker
ENTRYPOINT ["/app/ft_hackthon-worker"]

# CLI runtime stage
FROM gcr.io/distroless/static-debian12:nonroot AS cli
WORKDIR /app
COPY --from=builder /app/bin/ft_hackthon-cli /app/ft_hackthon-cli
USER nonroot:nonroot
ENTRYPOINT ["/app/ft_hackthon-cli"]

# CLI standalone binary extraction (docker build --output=bin/ --target=cli-binary .)
FROM scratch AS cli-binary
COPY --from=builder /app/bin/ft_hackthon-cli /ft_hackthon
