package moderation

import (
	"strings"
	"testing"
	"time"
)

func TestGateBlocksProfanity(t *testing.T) {
	g := NewGate(NewLimiter())
	res := g.Check("ты блять крут", SurfaceFeedComment, 1, time.Now())
	if res.Allowed {
		t.Fatal("expected profanity block")
	}
	if res.Reason != ReasonProfanity {
		t.Fatalf("reason=%s", res.Reason)
	}
}

func TestGateBlocksLink(t *testing.T) {
	g := NewGate(NewLimiter())
	res := g.Check("смотри t.me/spam", SurfaceFeedComment, 1, time.Now())
	if res.Allowed || res.Reason != ReasonLink {
		t.Fatalf("expected link block, got %+v", res)
	}
}

func TestGateBlocksCriticalRU(t *testing.T) {
	g := NewGate(NewLimiter())
	res := g.Check("продаю героин", SurfaceFeedComment, 1, time.Now())
	if res.Allowed || !res.AlertAdmin {
		t.Fatalf("expected critical block with admin alert, got %+v", res)
	}
}

func TestGateTooLongComment(t *testing.T) {
	g := NewGate(NewLimiter())
	text := strings.Repeat("а", MaxFeedCommentRunes+1)
	res := g.Check(text, SurfaceFeedComment, 1, time.Now())
	if res.Allowed || res.Reason != ReasonTooLong {
		t.Fatalf("expected too long, got %+v", res)
	}
}

func TestExtractUGCText(t *testing.T) {
	got := ExtractUGCText("бег, 15 мин, инт. 3/5\n\nЖим лёжа")
	if got != "Жим лёжа" {
		t.Fatalf("got %q", got)
	}
}

func TestWrapUserContent(t *testing.T) {
	w := WrapUserContent("note", "hello")
	if !strings.Contains(w, "<note>") || !strings.Contains(w, "hello") {
		t.Fatalf("wrap failed: %q", w)
	}
}

func TestRateLimit(t *testing.T) {
	g := NewGate(NewLimiter())
	now := time.Now()
	for i := 0; i < 5; i++ {
		res := g.Check("ok", SurfaceFeedComment, 42, now)
		if !res.Allowed {
			t.Fatalf("unexpected block at %d: %+v", i, res)
		}
	}
	res := g.Check("ok", SurfaceFeedComment, 42, now)
	if res.Allowed || res.Reason != ReasonRateLimited {
		t.Fatalf("expected rate limit, got %+v", res)
	}
}

func TestCheckContentSkipsRateLimit(t *testing.T) {
	g := NewGate(NewLimiter())
	now := time.Now()
	for i := 0; i < 10; i++ {
		res := g.CheckContent("админ пост", SurfaceAdminPost, now)
		if !res.Allowed {
			t.Fatalf("unexpected block at %d: %+v", i, res)
		}
	}
}
