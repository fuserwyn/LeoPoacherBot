package rag

import "fmt"

// Channel — изолированный контекст RAG (личка с Лео vs общий чат стаи).
type Channel string

const (
	ChannelPersonalLeo Channel = "personal_leo"
	ChannelPackGroup   Channel = "pack_group"
)

// PersonalSessionID — личная переписка юзера с Лео (не смешивается с общим чатом).
func PersonalSessionID(userID, packChatID int64) string {
	return fmt.Sprintf("personal:%d:%d", userID, packChatID)
}

// PackGroupSessionID — общий чат стаи (все участники, один контекст на pack).
func PackGroupSessionID(packChatID int64) string {
	return fmt.Sprintf("pack_group:%d", packChatID)
}
