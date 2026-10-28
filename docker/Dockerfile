# syntax=docker/dockerfile:1

# ---- build stage -----------------------------------------------------------
# Keep GO_VERSION in sync with the `toolchain` directive in go.mod.
ARG GO_VERSION=1.27.0

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

# Dependencies are cached independently of the source code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/bot ./cmd/bot \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/cron ./cmd/cron \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/migrate ./cmd/migrate

# ---- runtime stage ---------------------------------------------------------
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S app \
 && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/bot /out/migrate /out/cron ./
COPY configs ./configs

# Run as a non-root user.
USER app

EXPOSE 9090

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9090/healthz >/dev/null || exit 1

ENTRYPOINT ["./bot"]
