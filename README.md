# muzlan

A Telegram bot for searching music links.

## Current Scope

- Telegram bot listens with polling.
- User sends a music search query.
- Bot replies transparently while it works.
- Bot returns up to 10 YouTube links as inline URL buttons.
- Cache and per-user rate limiting are in memory for now.

## Configuration

Fill in `.env`:

- `TELEGRAM_BOT_TOKEN`: token from Telegram BotFather.
- `BOT_YTDLP_AUTO_INSTALL`: set to `true` to let `go-ytdlp` install `yt-dlp` automatically when the bot starts.

## Run

```sh
go run ./cmd/muzlan-bot
```

## Test

```sh
go test ./...
```
