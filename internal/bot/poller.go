package bot

import (
	"context"
	"log"
	"time"

	"github.com/dehobitto/muzlan/internal/telegram"
)

type Poller struct {
	client         *telegram.Client
	timeout        time.Duration
	skipOldUpdates bool
	logger         *log.Logger
}

func NewPoller(client *telegram.Client, timeout time.Duration, skipOldUpdates bool, logger *log.Logger) *Poller {
	return &Poller{
		client:         client,
		timeout:        timeout,
		skipOldUpdates: skipOldUpdates,
		logger:         logger,
	}
}

func (p *Poller) Run(ctx context.Context, handle func(context.Context, telegram.Update)) error {
	offset := 0
	if p.skipOldUpdates {
		last, err := p.client.GetUpdates(ctx, telegram.GetUpdatesRequest{
			Offset:  -1,
			Limit:   1,
			Timeout: 0,
		})
		if err != nil {
			return err
		}
		if len(last) > 0 {
			offset = last[len(last)-1].UpdateID + 1
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		updates, err := p.client.GetUpdates(ctx, telegram.GetUpdatesRequest{
			Offset:  offset,
			Limit:   50,
			Timeout: int(p.timeout.Seconds()),
		})
		if err != nil {
			p.logger.Printf("get updates: %v", err)
			sleep(ctx, time.Second)
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			handle(ctx, update)
		}
	}
}

func sleep(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
