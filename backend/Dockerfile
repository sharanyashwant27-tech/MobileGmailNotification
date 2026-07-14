# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
  && adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/server /app/server
COPY migrations /app/migrations
COPY web /app/web
ENV APP_ENV=production \
    APP_PORT=8088 \
    FIREBASE_CREDENTIALS_FILE=/app/credentials/firebase-service-account.json
EXPOSE 8088
USER appuser
HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8088/healthz || exit 1
ENTRYPOINT ["/app/server"]
