# syntax=docker/dockerfile:1
FROM golang:1.20 as builder
WORKDIR /build
ADD . /build/
# Cache for the modules
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

RUN --mount=type=cache,target=/root/.cache/go-build make build-for-docker

FROM alpine

RUN apk add --no-cache libgcc libstdc++ libc6-compat
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/sync-proxy /app/sync-proxy
ENTRYPOINT ["/app/sync-proxy"]
