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
	res := g.checkContent(text, surface)
	if !res.Allowed {
		return res
	}
	if g.limiter != nil && !g.limiter.Allow(surface, userID, now) {
		max := MaxRunes(surface)
		return blocked(
			ReasonRateLimited,
			warningFor(ReasonRateLimited, max),
			apiCodeFor(ReasonRateLimited),
			false,
		)
	}
	return allowed()
}

// CheckContent — PRE без rate-limit (админ-публикации).
func (g *Gate) CheckContent(text string, surface Surface, now time.Time) Result {
	_ = now
	return g.checkContent(text, surface)
}

func (g *Gate) checkContent(text string, surface Surface) Result {
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
