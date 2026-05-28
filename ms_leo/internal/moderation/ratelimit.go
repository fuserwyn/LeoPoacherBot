package moderation

import (
	"fmt"
	"sync"
	"time"
)

type rateRule struct {
	limit  int
	window time.Duration
}

var surfaceRateRules = map[Surface]rateRule{
	SurfaceTrainingNote:  {limit: 3, window: time.Minute},
	SurfaceFeedComment:   {limit: 5, window: time.Minute},
	SurfacePackGroupChat: {limit: 10, window: time.Minute},
}

// Limiter — in-memory sliding window (F1, один инстанс).
type Limiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{entries: make(map[string][]time.Time)}
}

func (l *Limiter) key(surface Surface, userID int64) string {
	return fmt.Sprintf("%d:%d", surface, userID)
}

func (l *Limiter) Allow(surface Surface, userID int64, now time.Time) bool {
	rule, ok := surfaceRateRules[surface]
	if !ok || userID == 0 || rule.limit <= 0 {
		return true
	}
	k := l.key(surface, userID)
	cutoff := now.Add(-rule.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	prev := l.entries[k]
	kept := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rule.limit {
		l.entries[k] = kept
		return false
	}
	kept = append(kept, now)
	l.entries[k] = kept
	return true
}
