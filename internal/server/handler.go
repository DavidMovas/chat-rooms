package server

import (
	"context"
	"fmt"
	"github.com/DavidMovas/chat-rooms/internal/config"
	"github.com/DavidMovas/chat-rooms/internal/log"
	"io"
	"log/slog"
	"time"

	"github.com/DavidMovas/chat-rooms/apis/chat"
)

var _ chat.ChatServiceServer = (*ChatServer)(nil)

type ChatServer struct {
	store *Store

	isLocal bool

	// UnimplementedChatServiceServer must be embedded to have forwarded compatible implementations.
	chat.UnimplementedChatServiceServer
}

func NewChatServer(store *Store, cfg *config.Config) *ChatServer {
	return &ChatServer{
		store:   store,
		isLocal: cfg.Local,
	}
}

func (s *ChatServer) CreateRoom(ctx context.Context, request *chat.CreateRoomRequest) (*chat.CreateRoomResponse, error) {
	room, err := s.store.CreateRoom(ctx, request.UserId, request.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	if s.isLocal {
		slog.Info("room created", "room_id", room.ID, "owner_id", room.OwnerID, "name", room.Name)
	}

	return &chat.CreateRoomResponse{
		RoomId: room.ID,
	}, nil
}

func (s *ChatServer) Connect(stream chat.ChatService_ConnectServer) error {
	ctx := stream.Context()

	in, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive message: %w", err)
	}

	connectRoom, ok := in.Payload.(*chat.ConnectRequest_ConnectRoom_)
	if !ok {
		return fmt.Errorf("failed to receive message: %w", err)
	}

	if s.isLocal {
		slog.Info("connect", "room_id", connectRoom.ConnectRoom.RoomId, "user_id", connectRoom.ConnectRoom.UserId)
	}

	hub, err := s.store.GetRoomHub(stream.Context(), connectRoom.ConnectRoom.RoomId)
	if err != nil {
		return fmt.Errorf("failed to get room hub: %w", err)
	}

	connection := hub.Connect(connectRoom.ConnectRoom.UserId, connectRoom.ConnectRoom.LastReadMessageNumber)
	defer connection.Disconnect()

	if err = stream.Send(&chat.ConnectResponse{
		Payload: &chat.ConnectResponse_MessageList{
			MessageList: mapToAPIMessageList(connection.Unread),
		},
	}); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	go func() {
		for m := range connection.MessagesCh {
			if err = stream.Send(&chat.ConnectResponse{
				Payload: &chat.ConnectResponse_Message{
					Message: mapToAPIMessage(m),
				},
			}); err != nil {
				log.FromContext(ctx).Error("failed to send message", "error", err)
			}
		}
	}()

	for {
		in, err = stream.Recv()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return fmt.Errorf("failed to receive message: %w", err)
		case stream.Context().Err() != nil:
			return stream.Context().Err()
		}

		switch p := in.Payload.(type) {
		case *chat.ConnectRequest_SendMessage_:
			msg := &Message{
				UserID:    connection.UserID,
				RoomID:    connection.RoomID,
				Text:      p.SendMessage.Text,
				CreatedAt: time.Now(),
			}
			if err = hub.ReceiveMessage(ctx, msg); err != nil {
				return fmt.Errorf("failed to receive message: %w", err)
			}
		default:
			return fmt.Errorf("failed to receive message: %w", err)
		}
	}
}
