package search

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/dehobitto/muzlan/internal/spotify"
)

type Service struct {
	spotify *spotify.Client
	ttl     time.Duration
	now     func() time.Time
	mu      sync.Mutex
	cache   map[string]cachedTracks
}

type cachedTracks struct {
	expiresAt time.Time
	tracks    []spotify.Track
}

func NewService(spotifyClient *spotify.Client, ttl time.Duration, now func() time.Time) *Service {
	return &Service{
		spotify: spotifyClient,
		ttl:     ttl,
		now:     now,
		cache:   make(map[string]cachedTracks),
	}
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]spotify.Track, error) {
	key := normalize(query)
	if tracks, ok := s.get(key); ok {
		return tracks, nil
	}

	tracks, err := s.spotify.SearchTracks(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	s.set(key, tracks)
	return tracks, nil
}

func (s *Service) get(key string) ([]spotify.Track, bool) {
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

	return append([]spotify.Track(nil), entry.tracks...), true
}

func (s *Service) set(key string, tracks []spotify.Track) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] = cachedTracks{
		expiresAt: s.now().Add(s.ttl),
		tracks:    append([]spotify.Track(nil), tracks...),
	}
}

func normalize(query string) string {
	return strings.Join(strings.Fields(strings.ToLower(query)), " ")
}
