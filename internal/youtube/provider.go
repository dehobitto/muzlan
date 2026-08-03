package youtube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	ytdlp "github.com/lrstanley/go-ytdlp"

	"github.com/dehobitto/muzlan/internal/search"
)

type Provider struct {
	autoInstall bool
}

type AudioStream struct {
	Reader   io.ReadCloser
	Filename string
	wait     func() error
	waitOnce sync.Once
	waitErr  error
}

func (s *AudioStream) Close() error {
	if s == nil || s.Reader == nil {
		return nil
	}
	return s.Reader.Close()
}

func (s *AudioStream) Wait() error {
	if s == nil || s.wait == nil {
		return nil
	}

	s.waitOnce.Do(func() {
		s.waitErr = s.wait()
	})
	return s.waitErr
}

type DownloadError struct {
	URL      string
	Err      error
	ExitCode int
	Stdout   string
	Stderr   string
	Files    []string
}

func (e DownloadError) Error() string {
	return fmt.Sprintf("download audio: %v", e.Err)
}

func (e DownloadError) Unwrap() error {
	return e.Err
}

func (e DownloadError) Diagnostics() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("url=%q", e.URL))
	if e.ExitCode != 0 {
		parts = append(parts, fmt.Sprintf("exit_code=%d", e.ExitCode))
	}
	if strings.TrimSpace(e.Stdout) != "" {
		parts = append(parts, "stdout="+strings.TrimSpace(e.Stdout))
	}
	if strings.TrimSpace(e.Stderr) != "" {
		parts = append(parts, "stderr="+strings.TrimSpace(e.Stderr))
	}
	if len(e.Files) > 0 {
		parts = append(parts, "files="+strings.Join(e.Files, ", "))
	}
	parts = append(parts, fmt.Sprintf("error=%v", e.Err))
	return strings.Join(parts, " ")
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
		JsRuntimes("node").
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

func (p *Provider) StreamAudio(ctx context.Context, videoURL string) (*AudioStream, error) {
	cmd := ytdlp.New().
		NoPlaylist().
		Format("bestaudio[ext=m4a]/bestaudio/best").
		JsRuntimes("node").
		Output("-").
		BuildCommand(ctx, videoURL)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, search.TemporaryError{Err: newStreamDownloadError(videoURL, cmd, stderr.String(), err)}
	}

	return &AudioStream{
		Reader:   stdout,
		Filename: "audio.m4a",
		wait: func() error {
			if err := cmd.Wait(); err != nil {
				return search.TemporaryError{Err: newStreamDownloadError(videoURL, cmd, stderr.String(), err)}
			}
			return nil
		},
	}, nil
}

func (p *Provider) DownloadMP3(ctx context.Context, videoURL string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "muzlan-audio-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	result, err := ytdlp.New().
		NoPlaylist().
		Format("bestaudio[ext=m4a]/bestaudio/best").
		JsRuntimes("node").
		RestrictFilenames().
		NoPart().
		Output(filepath.Join(dir, "%(title).120B-%(id)s.%(ext)s")).
		Run(ctx, videoURL)
	if err != nil {
		if path, audioErr := firstAudioFile(dir); audioErr == nil {
			return path, cleanup, nil
		}

		files := listFiles(dir)
		cleanup()
		return "", nil, search.TemporaryError{Err: newDownloadError(videoURL, result, files, err)}
	}

	path, err := firstAudioFile(dir)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return path, cleanup, nil
}

func newStreamDownloadError(videoURL string, cmd *exec.Cmd, stderr string, err error) DownloadError {
	exitCode := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if cmd != nil && cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	return DownloadError{
		URL:      videoURL,
		Err:      err,
		ExitCode: exitCode,
		Stderr:   stderr,
	}
}

func newDownloadError(videoURL string, result *ytdlp.Result, files []string, err error) DownloadError {
	if result == nil {
		return DownloadError{
			URL:   videoURL,
			Err:   err,
			Files: files,
		}
	}

	return DownloadError{
		URL:      videoURL,
		Err:      err,
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		Files:    files,
	}
}

func listFiles(dir string) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}

		label := filepath.Base(path)
		if info, infoErr := entry.Info(); infoErr == nil {
			label = fmt.Sprintf("%s(%d bytes)", label, info.Size())
		}
		files = append(files, label)
		return nil
	})
	return files
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

func firstAudioFile(dir string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || isTemporaryDownloadFile(path) {
			return nil
		}
		found = path
		return fs.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("yt-dlp did not produce an audio file")
	}
	return found, nil
}

func isTemporaryDownloadFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, ".part") ||
		strings.HasSuffix(name, ".ytdl") ||
		strings.HasSuffix(name, ".temp") ||
		strings.HasSuffix(name, ".tmp")
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
