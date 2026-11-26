package bot

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/dehobitto/muzlan/internal/search"
	"github.com/dehobitto/muzlan/internal/telegram"
)

// richResults renders one page of search results as Telegram rich HTML: the
// query (and page number) in a bordered frame, then a bordered table per video
// with number, title, channel and duration, and a full-width download button
// under it (a button inside a table cell is small). With thumbnails, each
// table has the video thumbnail above it (Telegram drops images placed inside
// cells) and tracks are split by a line; without them, by an empty line.
// Page buttons come last when there is more than one page.
func richResults(p resultsPage, thumbnails bool, download func(search.Result) string, nav func(page int) string) telegram.InputRichMessage {
	var b strings.Builder
	var media []telegram.InputRichMessageMedia
	// No CSS in rich HTML: padding comes from empty lines and centering.
	fmt.Fprintf(&b, `<table bordered><tr><td align="center">&nbsp;<br><b>%s</b>`,
		html.EscapeString(fmt.Sprintf(msgRichResultsTitle, p.query)))
	if p.pages > 1 {
		fmt.Fprintf(&b, "<br><sub>%d / %d</sub>", p.page, p.pages)
	}
	b.WriteString("<br>&nbsp;</td></tr></table>")

	separator := "<p>&nbsp;</p>"
	if thumbnails {
		separator = "<hr/>"
	}
	for i, result := range p.results {
		if i > 0 {
			b.WriteString(separator)
		}
		if thumbnails && result.Thumbnail != "" {
			id := fmt.Sprintf("thumb%d", i)
			media = append(media, telegram.InputRichMessageMedia{ID: id, Media: telegram.InputMedia{Type: "photo", Media: result.Thumbnail}})
			fmt.Fprintf(&b, `<img src="tg://photo?id=%s"/>`, id)
		}
		fmt.Fprintf(&b, "<table bordered><tr><td>%02d</td><td><b>%s</b>", p.first+i, html.EscapeString(result.Title))
		var details []string
		if result.Creator != "" {
			details = append(details, html.EscapeString(result.Creator))
		}
		if result.Duration > 0 {
			details = append(details, formatDuration(result.Duration))
		}
		if len(details) > 0 {
			fmt.Fprintf(&b, "<br><sub>%s</sub>", strings.Join(details, " · "))
		}
		fmt.Fprintf(&b, `</td></tr></table><tg-button-row>%s</tg-button-row>`,
			richButton("primary", download(result), msgRichDownload))
	}

	if p.pages > 1 {
		b.WriteString(separator + `<tg-button-row align="center">`)
		if p.page > 1 {
			b.WriteString(richButton("", nav(p.page-1), msgPagePrev))
		}
		if p.page < p.pages {
			b.WriteString(richButton("", nav(p.page+1), msgPageNext))
		}
		b.WriteString("</tg-button-row>")
	}
	return telegram.InputRichMessage{HTML: b.String(), Media: media}
}

func richButton(style string, data string, text string) string {
	if style != "" {
		style = fmt.Sprintf(` style="%s"`, style)
	}
	return fmt.Sprintf(`<tg-button type="callback_data"%s data="%s">%s</tg-button>`,
		style, html.EscapeString(data), html.EscapeString(text))
}

// formatDuration renders 447s as "7:27" and 3725s as "1:02:05".
func formatDuration(d time.Duration) string {
	total := int(d.Round(time.Second) / time.Second)
	h, m, s := total/3600, total/60%60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
