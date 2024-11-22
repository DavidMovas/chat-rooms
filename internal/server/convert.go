package server

import (
	"github.com/DavidMovas/chat-rooms/apis/chat"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapToAPIMessage(m *Message) *chat.Message {
	return &chat.Message{
		Number:    int64(m.Number),
		RoomId:    m.RoomID,
		UserId:    m.UserID,
		Text:      m.Text,
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
}

func mapToAPIMessageList(messages []*Message) *chat.MessageList {
	apiMessages := make([]*chat.Message, len(messages))
	for i, m := range messages {
		apiMessages[i] = mapToAPIMessage(m)
	}

	return &chat.MessageList{
		Messages: apiMessages,
	}
}
