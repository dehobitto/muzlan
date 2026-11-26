// Package stats records bot events to a JSON Lines file and summarizes them.
package stats

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	TypeSearch   = "search"
	TypeDownload = "download"
	TypePage     = "page"
	// TypeRichFallback marks a results message sent in a fallback layout.
	TypeRichFallback = "rich_fallback"
)

// Download failure reasons.
const (
	ReasonSignIn   = "youtube_sign_in"
	ReasonTimeout  = "timeout"
	ReasonYouTube  = "youtube"
	ReasonTelegram = "telegram"
)

// Event is one line of the events file. User is an HMAC of the Telegram user
// ID, so users can be counted but not identified.
type Event struct {
	Time    time.Time `json:"t"`
	Type    string    `json:"type"`
	User    string    `json:"user,omitempty"`
	OK      bool      `json:"ok,omitempty"`
	Reason  string    `json:"reason,omitempty"`
	Results int       `json:"results,omitempty"`
	MS      int64     `json:"ms,omitempty"`
	Level   string    `json:"level,omitempty"`
}

// Recorder appends events to a file. A nil Recorder records nothing.
type Recorder struct {
	mu   sync.Mutex
	path string
	salt []byte
	now  func() time.Time
}

// Open returns a Recorder writing to path, creating a random salt file next
// to it on first use. An empty path disables recording and returns nil.
func Open(path string) (*Recorder, error) {
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	salt, err := loadSalt(path + ".salt")
	if err != nil {
		return nil, err
	}
	return &Recorder{path: path, salt: salt, now: time.Now}, nil
}

func loadSalt(path string) ([]byte, error) {
	salt, err := os.ReadFile(path)
	if err == nil && len(salt) > 0 {
		return salt, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	salt = []byte(hex.EncodeToString(random[:]))
	return salt, os.WriteFile(path, salt, 0o600)
}

// Path returns the events file path, or "" for a nil Recorder.
func (r *Recorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// User returns the anonymous ID stored in events for a Telegram user ID.
func (r *Recorder) User(id int64) string {
	if r == nil {
		return ""
	}
	mac := hmac.New(sha256.New, r.salt)
	mac.Write([]byte(strconv.FormatInt(id, 10)))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

// Record appends event, stamping its time when unset.
func (r *Recorder) Record(event Event) error {
	if r == nil {
		return nil
	}
	if event.Time.IsZero() {
		event.Time = r.now().UTC()
	}
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// Load reads all events from path. A missing file has no events; malformed
// lines are skipped so one bad write does not hide the rest.
func Load(path string) ([]Event, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	return events, nil
}
