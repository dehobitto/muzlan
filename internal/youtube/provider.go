package youtube

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	if _, err := ytdlp.InstallFFmpeg(ctx, nil); err != nil {
		return fmt.Errorf("prepare ffmpeg: %w", err)
	}
	if _, err := ytdlp.InstallFFprobe(ctx, nil); err != nil {
		return fmt.Errorf("prepare ffprobe: %w", err)
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

func (p *Provider) DownloadMP3(ctx context.Context, videoURL string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "muzlan-audio-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	_, err = ytdlp.New().
		NoPlaylist().
		ExtractAudio().
		AudioFormat("mp3").
		AudioQuality("0").
		RestrictFilenames().
		NoPart().
		Output(filepath.Join(dir, "%(title).120B-%(id)s.%(ext)s")).
		Run(ctx, videoURL)
	if err != nil {
		cleanup()
		return "", nil, search.TemporaryError{Err: err}
	}

	path, err := firstMP3(dir)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return path, cleanup, nil
}

func IsYouTubeURL(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "https://www.youtube.com/watch?") ||
		strings.HasPrefix(normalized, "https://youtube.com/watch?") ||
		strings.HasPrefix(normalized, "https://youtu.be/") ||
		strings.HasPrefix(normalized, "http://www.youtube.com/watch?") ||
		strings.HasPrefix(normalized, "http://youtube.com/watch?") ||
		strings.HasPrefix(normalized, "http://youtu.be/")
}

func firstMP3(dir string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".mp3" {
			return nil
		}
		found = path
		return fs.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("yt-dlp did not produce an mp3 file")
	}
	return found, nil
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
