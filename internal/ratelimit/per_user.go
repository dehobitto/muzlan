package ratelimit

import (
	"sync"
	"time"
)

type PerUser struct {
	cooldown time.Duration
	now      func() time.Time
	mu       sync.Mutex
	lastSeen map[int64]time.Time
}

func NewPerUser(cooldown time.Duration, now func() time.Time) *PerUser {
	return &PerUser{
		cooldown: cooldown,
		now:      now,
		lastSeen: make(map[int64]time.Time),
	}
}

func (p *PerUser) Allow(userID int64) bool {
	if p.cooldown <= 0 {
		return true
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	current := p.now()
	last, ok := p.lastSeen[userID]
	if ok && current.Sub(last) < p.cooldown {
		return false
	}

	p.lastSeen[userID] = current
	return true
}
