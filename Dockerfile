# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for go get commands
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Tidy and build binaries
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/ft_hackthon-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/ft_hackthon-worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/ft_hackthon-cli ./cmd/ft_hackthon

# API runtime stage
FROM alpine:latest AS api
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/bin/ft_hackthon-api /app/ft_hackthon-api
EXPOSE 8000
CMD ["/app/ft_hackthon-api"]

# Worker runtime stage
FROM alpine:latest AS worker
WORKDIR /app
RUN apk add --no-cache ca-certificates git docker-cli
COPY --from=builder /app/bin/ft_hackthon-worker /app/ft_hackthon-worker
CMD ["/app/ft_hackthon-worker"]

# CLI runtime stage
FROM alpine:latest AS cli
WORKDIR /app
RUN apk add --no-cache ca-certificates git
COPY --from=builder /app/bin/ft_hackthon-cli /app/ft_hackthon-cli
ENTRYPOINT ["/app/ft_hackthon-cli"]
