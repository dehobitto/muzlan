# muzlan

A Telegram bot for searching music links.

## Current Scope

- Telegram bot listens with polling.
- User sends a music search query.
- Bot replies transparently while it works.
- Bot returns up to 10 YouTube links as inline URL buttons.
- Configured admins receive inline conversion buttons instead of link buttons.
- When an admin taps a result, the bot sends an MP3 for the authorized video.
- Cache and per-user rate limiting are in memory for now.

## Configuration

Fill in `.env`:

- `TELEGRAM_BOT_TOKEN`: token from Telegram BotFather.
- `BOT_ADMIN_USER_IDS`: comma-separated Telegram user IDs allowed to convert authorized URLs to MP3.
- `BOT_YTDLP_AUTO_INSTALL`: set to `true` to let `go-ytdlp` install `yt-dlp`, `ffmpeg`, and `ffprobe` automatically when the bot starts.

MP3 conversion requires `ffmpeg` to be available to `yt-dlp`.

## Run

```sh
go run ./cmd/muzlan-bot
```

## Test

```sh
go test ./...
```
