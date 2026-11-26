package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"github.com/dehobitto/muzlan/internal/stats"
	"github.com/dehobitto/muzlan/internal/telegram"
	"github.com/dehobitto/muzlan/internal/youtube"
)

const (
	statsDays     = 14
	statsBarWidth = 12
	alertInterval = time.Hour
)

func (h *Handler) record(event stats.Event) {
	if err := h.events.Record(event); err != nil {
		h.logger.Printf("record %s event: %v", event.Type, err)
	}
}

func (h *Handler) recordDownload(ctx context.Context, userID int64, start time.Time, failure string) {
	h.record(stats.Event{
		Type:   stats.TypeDownload,
		User:   h.events.User(userID),
		OK:     failure == "",
		Reason: failure,
		MS:     time.Since(start).Milliseconds(),
	})
	if failure == stats.ReasonSignIn {
		h.alertSignIn(ctx)
	}
}

func failureReason(err error) string {
	switch {
	case isSignInError(err):
		return stats.ReasonSignIn
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		return stats.ReasonTimeout
	default:
		return stats.ReasonYouTube
	}
}

// isSignInError reports YouTube's "confirm you're not a bot" block, which
// means the VM's exit IP (Cloudflare WARP) is blocked.
func isSignInError(err error) bool {
	var downloadErr youtube.DownloadError
	return errors.As(err, &downloadErr) && strings.Contains(downloadErr.Stderr, "Sign in to confirm")
}

// alertSignIn tells admins about the sign-in block, at most once per hour.
func (h *Handler) alertSignIn(ctx context.Context) {
	h.alertMu.Lock()
	if time.Since(h.lastAlert) < alertInterval {
		h.alertMu.Unlock()
		return
	}
	h.lastAlert = time.Now()
	h.alertMu.Unlock()

	for adminID := range h.cfg.AdminUserIDs {
		if _, err := h.client.SendMessage(ctx, telegram.SendMessageRequest{ChatID: adminID, Text: msgAlertSignIn}); err != nil {
			h.logger.Printf("send sign-in alert: %v", err)
		}
	}
}

// handleStats sends the summary as a rich message, then the events file
// itself as a backup.
func (h *Handler) handleStats(ctx context.Context, chatID int64) {
	if h.events == nil {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{ChatID: chatID, Text: msgStatsDisabled})
		return
	}

	events, err := stats.Load(h.events.Path())
	if err != nil {
		h.logger.Printf("load events: %v", err)
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{ChatID: chatID, Text: msgStatsFailed})
		return
	}

	summary := stats.Summarize(events, time.Now(), statsDays)
	if _, err := h.client.SendRichMessage(ctx, telegram.SendRichMessageRequest{ChatID: chatID, RichMessage: richStats(summary)}); err != nil {
		h.logger.Printf("send stats: %v", err)
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{ChatID: chatID, Text: msgStatsFailed})
	}

	// Telegram rejects empty files.
	if info, err := os.Stat(h.events.Path()); err != nil || info.Size() == 0 {
		return
	}
	if _, err := h.client.SendDocument(ctx, telegram.SendDocumentRequest{
		ChatID:       chatID,
		DocumentPath: h.events.Path(),
		Caption:      fmt.Sprintf(msgStatsFile, len(events)),
	}); err != nil {
		h.logger.Printf("send events file: %v", err)
	}
}

func richStats(s stats.Summary) telegram.InputRichMessage {
	var b strings.Builder
	fmt.Fprintf(&b, `<table bordered><tr><td align="center">&nbsp;<br><b>%s</b><br>&nbsp;</td></tr></table>`, msgStatsTitle)

	fmt.Fprintf(&b, "<table striped><tr><th></th><th>%s</th><th>%s</th><th>%s</th></tr>", msgStatsToday, msgStatsWeek, msgStatsAll)
	for _, row := range []struct {
		label  string
		values [3]int
	}{
		{msgStatsDownloads, s.Downloads},
		{msgStatsFailedRow, s.Failed},
		{msgStatsUsers, s.Users},
		{msgStatsNewUsers, s.NewUsers},
		{msgStatsSearches, s.Searches},
	} {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>", row.label, row.values[0], row.values[1], row.values[2])
	}
	b.WriteString("</table>")

	fmt.Fprintf(&b, "<h4>%s</h4><table>", fmt.Sprintf(msgStatsDaily, statsDays))
	most := 0
	for _, day := range s.DailyDownloads {
		most = max(most, day.Count)
	}
	for _, day := range s.DailyDownloads {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%d</td></tr>", day.Day.Format("01.02"), bar(day.Count, most), day.Count)
	}
	b.WriteString("</table>")

	reasons := []struct{ label, key string }{
		{msgStatsReasonSignIn, stats.ReasonSignIn},
		{msgStatsReasonTimeout, stats.ReasonTimeout},
		{msgStatsReasonYouTube, stats.ReasonYouTube},
		{msgStatsReasonTelegram, stats.ReasonTelegram},
	}
	failures := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		failures = append(failures, fmt.Sprintf("%s %d", reason.label, s.Failures[reason.key]))
	}
	fmt.Fprintf(&b, "<p><b>%s</b> %s</p>", msgStatsFailures, html.EscapeString(strings.Join(failures, " · ")))
	fmt.Fprintf(&b, "<p><b>%s</b> %s %s / %s · %s %s / %s <sub>%s</sub></p>", msgStatsTime,
		msgStatsSearch, seconds(s.SearchMedian), seconds(s.SearchP95),
		msgStatsDownload, seconds(s.DownloadMedian), seconds(s.DownloadP95), msgStatsTimeNote)

	return telegram.InputRichMessage{HTML: b.String()}
}

// bar draws count as up to statsBarWidth blocks relative to most; any
// non-zero count gets at least one block.
func bar(count int, most int) string {
	if count == 0 || most == 0 {
		return ""
	}
	return strings.Repeat("█", max(1, (count*statsBarWidth+most/2)/most))
}

func seconds(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}
