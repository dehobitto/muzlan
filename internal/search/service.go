package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Service struct {
	provider Provider
	ttl      time.Duration
	now      func() time.Time
	mu       sync.Mutex
	cache    map[string]cachedResults
}

type Provider interface {
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

type Result struct {
	Title   string
	Creator string
	URL     string
}

func (r Result) Label() string {
	if r.Creator == "" {
		return r.Title
	}
	return fmt.Sprintf("%s - %s", r.Title, r.Creator)
}

type TemporaryError struct {
	Err error
}

func (e TemporaryError) Error() string {
	return e.Err.Error()
}

func (e TemporaryError) Unwrap() error {
	return e.Err
}

func IsTemporary(err error) bool {
	var temporary TemporaryError
	return errors.As(err, &temporary)
}

type cachedResults struct {
	expiresAt time.Time
	results   []Result
}

func NewService(provider Provider, ttl time.Duration, now func() time.Time) *Service {
	return &Service{
		provider: provider,
		ttl:      ttl,
		now:      now,
		cache:    make(map[string]cachedResults),
	}
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	key := normalize(query)
	if results, ok := s.get(key); ok {
		return results, nil
	}

	results, err := s.provider.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	s.set(key, results)
	return results, nil
}

func (s *Service) get(key string) ([]Result, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.cache[key]
	if !ok {
		return nil, false
	}
	if !s.now().Before(entry.expiresAt) {
		delete(s.cache, key)
		return nil, false
	}

	return append([]Result(nil), entry.results...), true
}

func (s *Service) set(key string, results []Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] = cachedResults{
		expiresAt: s.now().Add(s.ttl),
		results:   append([]Result(nil), results...),
	}
}

func normalize(query string) string {
	return strings.Join(strings.Fields(strings.ToLower(query)), " ")
}
