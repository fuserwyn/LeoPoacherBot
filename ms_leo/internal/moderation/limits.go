package moderation

import "unicode/utf8"

const (
	MaxTrainingNoteRunes      = 1500
	MaxFeedCommentRunes       = 500
	MaxPackGroupRunes         = 4000
	MaxAdminPostRunes         = 4000
	MaxAdminPollQuestionRunes = 300
	MaxAdminPollOptionRunes   = 100
	MaxLeoChatRunes           = 4000
	MaxLeoChatMessagesPerDay  = 20
)

func MaxRunes(surface Surface) int {
	switch surface {
	case SurfaceTrainingNote:
		return MaxTrainingNoteRunes
	case SurfaceFeedComment:
		return MaxFeedCommentRunes
	case SurfacePackGroupChat:
		return MaxPackGroupRunes
	case SurfaceAdminPost:
		return MaxAdminPostRunes
	case SurfaceAdminPollQuestion:
		return MaxAdminPollQuestionRunes
	case SurfaceAdminPollOption:
		return MaxAdminPollOptionRunes
	case SurfaceLeoChat:
		return MaxLeoChatRunes
	default:
		return MaxPackGroupRunes
	}
}

func runeCount(s string) int {
	return utf8.RuneCountInString(s)
}
