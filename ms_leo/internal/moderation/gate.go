package moderation

import (
	"strings"
	"time"
)

// Gate — синхронный PRE-фильтр до публикации и до LLM.
type Gate struct {
	limiter *Limiter
}

func NewGate(limiter *Limiter) *Gate {
	if limiter == nil {
		limiter = NewLimiter()
	}
	return &Gate{limiter: limiter}
}

func (g *Gate) Check(text string, surface Surface, userID int64, now time.Time) Result {
	text = strings.TrimSpace(text)
	max := MaxRunes(surface)
	if text == "" {
		return allowed()
	}
	if runeCount(text) > max {
		return blocked(
			ReasonTooLong,
			warningFor(ReasonTooLong, max),
			apiCodeFor(ReasonTooLong),
			false,
		)
	}
	if g.limiter != nil && !g.limiter.Allow(surface, userID, now) {
		return blocked(
			ReasonRateLimited,
			warningFor(ReasonRateLimited, max),
			apiCodeFor(ReasonRateLimited),
			false,
		)
	}
	if containsCriticalRU(text) {
		return blocked(
			ReasonCriticalRU,
			warningFor(ReasonCriticalRU, max),
			apiCodeFor(ReasonCriticalRU),
			true,
		)
	}
	if containsProfanity(text) {
		return blocked(
			ReasonProfanity,
			warningFor(ReasonProfanity, max),
			apiCodeFor(ReasonProfanity),
			false,
		)
	}
	if containsBlockedLink(text) {
		return blocked(
			ReasonLink,
			warningFor(ReasonLink, max),
			apiCodeFor(ReasonLink),
			false,
		)
	}
	return allowed()
}
