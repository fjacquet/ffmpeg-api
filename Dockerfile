# Local/dev build from source. Releases use Dockerfile.goreleaser (prebuilt binary).
FROM golang:1.27.1-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /ffmpeg-api .

FROM alpine:latest
RUN apk --no-cache add ffmpeg && adduser -D -u 10001 ffmpeg-api
COPY --from=builder /ffmpeg-api /usr/bin/ffmpeg-api
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/livez || exit 1
USER ffmpeg-api
ENTRYPOINT ["/usr/bin/ffmpeg-api"]
