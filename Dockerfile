# syntax=docker/dockerfile:1
FROM golang:1.18 as builder
WORKDIR /build
ADD . /build/
RUN --mount=type=cache,target=/root/.cache/go-build make build-for-docker

FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/builder-proxy /app/builder-proxy
ENV LISTEN_ADDR=":8080"
EXPOSE 8080
CMD ["/app/builder-proxy"]
