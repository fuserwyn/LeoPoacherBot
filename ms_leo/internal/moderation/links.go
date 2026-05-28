package moderation

import "regexp"

var (
	reURL = regexp.MustCompile(`(?i)(?:https?://|www\.|t\.me/|telegram\.me/|tg://)`)
	reAt  = regexp.MustCompile(`(?i)(?:^|[\s(\[,{«"'])@[a-zA-Z0-9_]{4,32}\b`)
)

func containsBlockedLink(text string) bool {
	if text == "" {
		return false
	}
	return reURL.MatchString(text) || reAt.MatchString(text)
}
