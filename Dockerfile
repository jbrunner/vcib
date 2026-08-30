FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o vcib ./cmd/vcib

FROM gcr.io/distroless/static-debian13:nonroot
LABEL org.opencontainers.image.source=https://github.com/jbrunner/vcib \
      org.opencontainers.image.title="Varnish Cache Invalidation Broker" \
      org.opencontainers.image.description="Forwards PURGE/BAN requests to all ready Varnish pods in a Kubernetes namespace" \
      org.opencontainers.image.licenses="MIT"
COPY --from=builder /app/vcib /vcib
ENTRYPOINT ["/vcib"]
