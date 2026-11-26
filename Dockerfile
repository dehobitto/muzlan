FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /muzlan-bot ./cmd/muzlan-bot

# Node.js is the JavaScript runtime for yt-dlp.
FROM node:26.13.1-bookworm-slim
ARG YTDLP_VERSION=2026.11.23
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && case "$(dpkg --print-architecture)" in \
         amd64) asset=yt-dlp_linux sum=249ddb058234276bc449b9cd9492aad9f133bda5923c61a9e39983ae114ef17d ;; \
         arm64) asset=yt-dlp_linux_aarch64 sum=fb847a2e4a9ddc8cf5fcfb4bb7f42ba8969ecf72068def0278074f9d300bd13c ;; \
         *) echo "unsupported architecture" >&2; exit 1 ;; \
       esac \
    && curl -fsSL -o /usr/local/bin/yt-dlp "https://github.com/yt-dlp/yt-dlp/releases/download/${YTDLP_VERSION}/${asset}" \
    && echo "${sum}  /usr/local/bin/yt-dlp" | sha256sum -c - \
    && chmod 755 /usr/local/bin/yt-dlp
COPY --from=build /muzlan-bot /usr/local/bin/muzlan-bot
ENV BOT_YTDLP_AUTO_INSTALL=false
USER node
WORKDIR /home/node
ENTRYPOINT ["muzlan-bot"]
