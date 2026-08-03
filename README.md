# muzlan

A Telegram bot for searching music links.

## Current Scope

- Telegram bot listens with polling.
- User sends a music search query.
- Bot replies transparently while it works.
- Bot returns up to 10 YouTube results as inline buttons.
- When a result is tapped, the bot streams audio from `yt-dlp` to Telegram.
- The happy path sends audio with Telegram player support.
- If Telegram rejects the streamed audio upload, the bot retries once as a document stream.
- Cache and per-user rate limiting are in memory for now.

## Configuration

Fill in `.env`:

- `TELEGRAM_BOT_TOKEN`: token from Telegram BotFather.
- `BOT_ADMIN_USER_IDS`: comma-separated Telegram user IDs for admin-only flows when they are enabled.
- `BOT_YTDLP_AUTO_INSTALL`: set to `true` to let `go-ytdlp` install `yt-dlp` automatically when the bot starts. Docker sets this to `false` because `yt-dlp` is installed in the image.

## Run

```sh
go run ./cmd/muzlan-bot
```

## Docker

The Docker image pins:

- Go image: `golang:1.25.0-bookworm`
- Runtime image with Node.js: `node:22.18.0-bookworm-slim`
- `yt-dlp`: `2026.07.04`

```sh
docker compose up --build
```

Node.js is included for `yt-dlp --js-runtimes node`.

## Test

```sh
go test ./...
```
