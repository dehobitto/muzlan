package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/dehobitto/muzlan/internal/bot"
	"github.com/dehobitto/muzlan/internal/config"
	"github.com/dehobitto/muzlan/internal/ratelimit"
	"github.com/dehobitto/muzlan/internal/search"
	"github.com/dehobitto/muzlan/internal/telegram"
	"github.com/dehobitto/muzlan/internal/youtube"
)

type App struct {
	cfg     config.Config
	updates *bot.Poller
	handler *bot.Handler
	youtube *youtube.Provider
	logger  *log.Logger
}

func New(cfg config.Config, httpClient *http.Client, logger *log.Logger) *App {
	telegramClient := telegram.NewClient(cfg.TelegramBotToken, httpClient)
	youtubeProvider := youtube.NewProvider(cfg.YTDLPAutoInstall)
	searcher := search.NewService(youtubeProvider, cfg.CacheTTL, time.Now)
	limiter := ratelimit.NewPerUser(cfg.UserCooldown, time.Now)

	handler := bot.NewHandler(bot.HandlerConfig{
		QueryMinLength: cfg.QueryMinLength,
		QueryMaxLength: cfg.QueryMaxLength,
		ResultLimit:    cfg.ResultLimit,
		SearchTimeout:  cfg.SearchTimeout,
		RetryCount:     cfg.RetryCount,
	}, telegramClient, searcher, limiter, logger)

	return &App{
		cfg:     cfg,
		updates: bot.NewPoller(telegramClient, cfg.PollTimeout, cfg.SkipOldUpdates, logger),
		handler: handler,
		youtube: youtubeProvider,
		logger:  logger,
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.cfg.Validate(); err != nil {
		return err
	}
	if err := a.youtube.Prepare(ctx); err != nil {
		return err
	}

	a.logger.Println("muzlan bot is listening")
	return a.updates.Run(ctx, a.handler.HandleUpdate)
}
