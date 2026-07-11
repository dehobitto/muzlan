package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dehobitto/muzlan/internal/app"
	"github.com/dehobitto/muzlan/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(".env")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}
	bot := app.New(cfg, httpClient, log.Default())

	if err := bot.Run(ctx); err != nil {
		log.Fatalf("run bot: %v", err)
	}
}
