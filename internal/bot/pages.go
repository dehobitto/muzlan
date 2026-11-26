package bot

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dehobitto/muzlan/internal/search"
)

const (
	pageCallbackPrefix = "page:"
	pageSize           = 5
)

// searchPages remembers the query behind each results message, so page
// buttons fit Telegram's 64-byte callback data. Results themselves come
// back from the search cache.
type searchPages struct {
	mu      sync.Mutex
	now     func() time.Time
	ttl     time.Duration
	entries map[string]pageEntry
}

type pageEntry struct {
	query     string
	expiresAt time.Time
}

func newSearchPages(ttl time.Duration, now func() time.Time) *searchPages {
	return &searchPages{now: now, ttl: ttl, entries: make(map[string]pageEntry)}
}

func (p *searchPages) put(query string) string {
	token := randomToken()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.entries[token] = pageEntry{query: query, expiresAt: p.now().Add(p.ttl)}
	return token
}

func (p *searchPages) get(token string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.entries[token]
	if !ok {
		return "", false
	}
	if !p.now().Before(entry.expiresAt) {
		delete(p.entries, token)
		return "", false
	}
	return entry.query, true
}

func pageCallback(token string, page int) string {
	return pageCallbackPrefix + token + ":" + strconv.Itoa(page)
}

func parsePageCallback(data string) (token string, page int, ok bool) {
	rest, found := strings.CutPrefix(data, pageCallbackPrefix)
	if !found {
		return "", 0, false
	}
	token, number, found := strings.Cut(rest, ":")
	page, err := strconv.Atoi(number)
	if !found || token == "" || err != nil || page < 1 {
		return "", 0, false
	}
	return token, page, true
}

// resultsPage is one page of search results; page and first count from 1.
type resultsPage struct {
	query   string
	results []search.Result
	first   int
	page    int
	pages   int
}

// newResultsPage cuts page out of results, clamping it to the last page.
func newResultsPage(query string, results []search.Result, page int) resultsPage {
	pages := max(1, (len(results)+pageSize-1)/pageSize)
	page = min(max(page, 1), pages)
	start := (page - 1) * pageSize
	end := min(start+pageSize, len(results))
	return resultsPage{query: query, results: results[start:end], first: start + 1, page: page, pages: pages}
}
