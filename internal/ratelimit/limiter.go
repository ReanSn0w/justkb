package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	count int
	until time.Time
}

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	items  map[string]entry
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, now: time.Now, items: make(map[string]entry)}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	item := l.items[key]
	if !now.Before(item.until) {
		item = entry{until: now.Add(l.window)}
	}
	if item.count >= l.limit {
		l.items[key] = item
		return false
	}
	item.count++
	l.items[key] = item
	return true
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.items, key)
	l.mu.Unlock()
}
