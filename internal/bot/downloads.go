package bot

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const downloadCallbackPrefix = "download:"

type downloadButtons struct {
	mu      sync.Mutex
	now     func() time.Time
	ttl     time.Duration
	entries map[string]downloadEntry
}

type downloadEntry struct {
	url       string
	title     string
	expiresAt time.Time
}

func newDownloadButtons(ttl time.Duration, now func() time.Time) *downloadButtons {
	return &downloadButtons{
		now:     now,
		ttl:     ttl,
		entries: make(map[string]downloadEntry),
	}
}

func (d *downloadButtons) put(url string, title string) string {
	token := randomToken()

	d.mu.Lock()
	defer d.mu.Unlock()

	d.entries[token] = downloadEntry{
		url:       url,
		title:     strings.TrimSpace(title),
		expiresAt: d.now().Add(d.ttl),
	}
	return downloadCallbackPrefix + token
}

func (d *downloadButtons) get(data string) (downloadEntry, bool) {
	if !strings.HasPrefix(data, downloadCallbackPrefix) {
		return downloadEntry{}, false
	}

	token := data[len(downloadCallbackPrefix):]

	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.entries[token]
	if !ok {
		return downloadEntry{}, false
	}
	if !d.now().Before(entry.expiresAt) {
		delete(d.entries, token)
		return downloadEntry{}, false
	}

	return entry, true
}

func randomToken() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes[:])
}
