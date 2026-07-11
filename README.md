# muzlan

A Telegram bot for searching music links.

## Current Scope

- Telegram bot listens with polling.
- User sends a Spotify search query.
- Bot replies transparently while it works.
- Bot returns up to 10 Spotify results as inline URL buttons.
- Cache and per-user rate limiting are in memory for now.

## Configuration

Fill in `.env`:

- `TELEGRAM_BOT_TOKEN`: token from Telegram BotFather.
- `SPOTIFY_CLIENT_ID`: Spotify app client ID.
- `SPOTIFY_CLIENT_SECRET`: Spotify app client secret.

## Run

```sh
go run ./cmd/muzlan-bot
```

## Test

```sh
go test ./...
```
