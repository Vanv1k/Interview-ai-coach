package ratelimit

import (
	"sync"
	"time"
)

type Limiter interface {
	Allow(key string, limit int, window time.Duration) bool
}

type Memory struct {
	now     func() time.Time
	mu      sync.Mutex
	buckets map[string]bucket
}

type bucket struct {
	count      int
	windowEnd  time.Time
	lastAccess time.Time
}

func NewMemory() *Memory {
	return &Memory{
		now:     time.Now,
		buckets: map[string]bucket{},
	}
}

func (m *Memory) Allow(key string, limit int, window time.Duration) bool {
	if limit <= 0 {
		return true
	}
	now := m.now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanup(now, window)
	b := m.buckets[key]
	if b.windowEnd.IsZero() || !now.Before(b.windowEnd) {
		m.buckets[key] = bucket{count: 1, windowEnd: now.Add(window), lastAccess: now}
		return true
	}
	if b.count >= limit {
		b.lastAccess = now
		m.buckets[key] = b
		return false
	}
	b.count++
	b.lastAccess = now
	m.buckets[key] = b
	return true
}

func (m *Memory) cleanup(now time.Time, window time.Duration) {
	if len(m.buckets) < 1000 {
		return
	}
	ttl := window * 2
	for key, b := range m.buckets {
		if now.Sub(b.lastAccess) > ttl {
			delete(m.buckets, key)
		}
	}
}
