package moderation

// Surface — UGC-поверхность для PRE-гейта.
type Surface int

const (
	SurfaceTrainingNote Surface = iota
	SurfaceFeedComment
	SurfacePackGroupChat
)

// Reason — причина блокировки PRE-слоя.
type Reason string

const (
	ReasonOK          Reason = "ok"
	ReasonProfanity   Reason = "profanity"
	ReasonLink        Reason = "link"
	ReasonCriticalRU  Reason = "critical_ru"
	ReasonTooLong     Reason = "too_long"
	ReasonRateLimited Reason = "rate_limited"
	ReasonMuted       Reason = "muted"
)

// Result — итог синхронной PRE-проверки.
type Result struct {
	Allowed     bool
	Reason      Reason
	UserMessage string
	AlertAdmin  bool
	APICode     string
}

func blocked(reason Reason, msg, apiCode string, alertAdmin bool) Result {
	return Result{
		Allowed:     false,
		Reason:      reason,
		UserMessage: msg,
		AlertAdmin:  alertAdmin,
		APICode:     apiCode,
	}
}

func allowed() Result {
	return Result{Allowed: true, Reason: ReasonOK}
}
