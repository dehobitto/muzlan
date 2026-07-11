package youtube

import (
	"context"
	"fmt"
	"strings"

	ytdlp "github.com/lrstanley/go-ytdlp"

	"github.com/dehobitto/muzlan/internal/search"
)

type Provider struct {
	autoInstall bool
}

func NewProvider(autoInstall bool) *Provider {
	return &Provider{autoInstall: autoInstall}
}

func (p *Provider) Prepare(ctx context.Context) error {
	if !p.autoInstall {
		return nil
	}

	if _, err := ytdlp.Install(ctx, nil); err != nil {
		return fmt.Errorf("prepare yt-dlp: %w", err)
	}

	return nil
}

func (p *Provider) Search(ctx context.Context, query string, limit int) ([]search.Result, error) {
	if limit < 1 {
		return nil, nil
	}

	result, err := ytdlp.New().
		FlatPlaylist().
		DumpJSON().
		NoWarnings().
		Run(ctx, fmt.Sprintf("ytsearch%d:%s", limit, query))
	if err != nil {
		return nil, search.TemporaryError{Err: err}
	}

	infos, err := result.GetExtractedInfo()
	if err != nil {
		return nil, err
	}

	results := make([]search.Result, 0, limit)
	seen := map[string]bool{}
	for _, info := range flattenInfos(infos) {
		link := resultFromInfo(info)
		if link.Title == "" || link.URL == "" || seen[link.URL] {
			continue
		}

		seen[link.URL] = true
		results = append(results, link)
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

func flattenInfos(infos []*ytdlp.ExtractedInfo) []*ytdlp.ExtractedInfo {
	var flattened []*ytdlp.ExtractedInfo
	for _, info := range infos {
		if info == nil {
			continue
		}
		if len(info.Entries) > 0 {
			flattened = append(flattened, flattenInfos(info.Entries)...)
			continue
		}
		flattened = append(flattened, info)
	}
	return flattened
}

func resultFromInfo(info *ytdlp.ExtractedInfo) search.Result {
	return search.Result{
		Title:   deref(info.Title),
		Creator: firstNonEmpty(deref(info.Artist), deref(info.Creator), deref(info.Uploader), deref(info.Channel)),
		URL:     youtubeURL(info),
	}
}

func youtubeURL(info *ytdlp.ExtractedInfo) string {
	if value := deref(info.WebpageURL); strings.HasPrefix(value, "http") {
		return value
	}
	if value := deref(info.URL); strings.HasPrefix(value, "http") && strings.Contains(value, "youtube.com/watch") {
		return value
	}
	if strings.TrimSpace(info.ID) != "" {
		return "https://www.youtube.com/watch?v=" + strings.TrimSpace(info.ID)
	}
	return ""
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
