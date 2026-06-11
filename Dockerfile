FROM golang:1.25-alpine AS builder
WORKDIR /src
# Cloud DEV builds run from China-hosted infrastructure; default to a reachable
# Go module proxy while keeping build args overrideable for other networks.
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/platform-service ./cmd/server

FROM alpine:3.19
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates tzdata wget
RUN addgroup -g 1000 -S appuser &&     adduser -u 1000 -S appuser -G appuser
WORKDIR /app
COPY --from=builder /out/platform-service ./platform-service
COPY config.*.yaml ./
RUN mkdir -p /app/data && chown -R appuser:appuser /app
USER appuser
ENV PLATFORM_PORT=8095
ENV PLATFORM_CONFIG_FILE=config.prod
EXPOSE 8095
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3   CMD wget --no-verbose --tries=1 --spider http://localhost:${PLATFORM_PORT}/healthz || exit 1
CMD ["./platform-service", "-config", "config.prod"]
