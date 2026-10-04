# ffmpeg-api

[![CI](https://github.com/fjacquet/ffmpeg-api/actions/workflows/ci.yml/badge.svg)](https://github.com/fjacquet/ffmpeg-api/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/fjacquet/ffmpeg-api?include_prereleases&sort=semver)](https://github.com/fjacquet/ffmpeg-api/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/fjacquet/ffmpeg-api)](go.mod)
[![License](https://img.shields.io/github/license/fjacquet/ffmpeg-api)](LICENSE)

Tiny internal HTTP sidecar that turns a WAV upload into an iOS-friendly AAC/M4A file.
Built to give n8n workflows (whose Code nodes cannot run binaries) access to ffmpeg —
for example to compress a Gemini TTS podcast small enough to attach to an email.

> **No authentication.** Run it only on a private Docker network, never publish its port.

## API

| Method | Path | Body | Response |
|---|---|---|---|
| `POST` | `/m4a` | WAV (≤ 60 MiB) | `audio/mp4` — AAC-LC 40 kbps mono 24 kHz, loudness-normalised to −16 LUFS, `+faststart` |
| `GET` | `/livez` | — | `ok` |

Optional `?filename=name.m4a` sets `Content-Disposition` (path components are stripped), so the n8n HTTP Request node names the file.

Errors: `413` body too large, `422` ffmpeg could not decode the input (body is logged server-side).
Encoding is bounded to 2 minutes per request. Roughly 1.9 MB per 6 minutes of speech.

## Docker Compose

```yaml
  ffmpeg-api:
    image: ghcr.io/fjacquet/ffmpeg-api:0.1
    restart: always
    mem_limit: 256m
    labels:
      - traefik.enable=false
```

From another container on the same network:

```bash
curl --data-binary @podcast.wav -o podcast.m4a http://ffmpeg-api:8080/m4a
```

From n8n, use an **HTTP Request node** (body: n8n Binary File, response format: File).
Do not post the audio from a Code node through `this.helpers.httpRequest`: with external
task runners the helper call is relayed as JSON, so a `Buffer` body arrives as a
`{"type":"Buffer","data":[...]}` array roughly four times its size (observed as a 413).

## Development

```bash
make tools   # golangci-lint, govulncheck, goreleaser
make ci      # lint + test + build + vuln
make docker  # local image from ./Dockerfile
```

Releases: push a `v*` tag; GoReleaser publishes the multi-arch image to `ghcr.io/fjacquet/ffmpeg-api`.

## License

Apache-2.0
