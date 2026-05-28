package moderation

import "unicode/utf8"

const (
	MaxTrainingNoteRunes = 1500
	MaxFeedCommentRunes  = 500
	MaxPackGroupRunes    = 4000
)

func MaxRunes(surface Surface) int {
	switch surface {
	case SurfaceTrainingNote:
		return MaxTrainingNoteRunes
	case SurfaceFeedComment:
		return MaxFeedCommentRunes
	case SurfacePackGroupChat:
		return MaxPackGroupRunes
	default:
		return MaxPackGroupRunes
	}
}

func runeCount(s string) int {
	return utf8.RuneCountInString(s)
}
