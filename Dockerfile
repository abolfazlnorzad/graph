FROM golang:1.25.5-alpine AS builder

RUN apk update && apk add --no-cache git ca-certificates tzdata

RUN adduser -D -g '' -u 1001 appuser

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor ./vendor

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -mod=vendor -a -installsuffix cgo -ldflags="-w -s" -o /app/bin/server cmd/main.go

# ==========================================
# Stage 2: Final (Production Image)
# ==========================================
FROM alpine:3.19

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/bin/server .
COPY --from=builder --chown=appuser:appuser /app/config.yml .
COPY --from=builder --chown=appuser:appuser /app/docs ./docs
COPY --from=builder --chown=appuser:appuser /app/repository/migrations ./migrations
RUN mkdir -p /app/logs && chown appuser:appuser /app/logs

USER appuser:appuser

EXPOSE 8080

CMD ["./server"]
