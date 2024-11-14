package server

import (
	"context"
	"github.com/DavidMovas/chat-rooms/apis/chat"
)

var _ chat.ChatServiceServer = (*Handler)(nil)

type Handler struct {
	store *Store

	// UnimplementedChatServiceServer must be embedded to have forward compatible implementations.
	chat.UnimplementedChatServiceServer
}

func NewChatServer(store *Store) *Handler {
	return &Handler{
		store: store,
	}
}

func (h Handler) CreateRoom(ctx context.Context, request *chat.CreateRoomRequest) (*chat.CreateRoomResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (h Handler) Connect(ctx context.Context, request *chat.ConnectRequest) (*chat.ConnectResponse, error) {
	//TODO implement me
	panic("implement me")
}
