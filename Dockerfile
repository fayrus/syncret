FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /app/syncret ./cmd/syncret

FROM cgr.dev/chainguard/static:latest
COPY --from=builder --chmod=755 /app/syncret /syncret
ENTRYPOINT ["/syncret"]
