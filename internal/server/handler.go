package server

import (
	"context"
	"fmt"
	"github.com/DavidMovas/chat-rooms/apis/chat"
	"io"
	"log/slog"
	"time"
)

var _ chat.ChatServiceServer = (*ChatServer)(nil)

type ChatServer struct {
	store *Store

	// UnimplementedChatServiceServer must be embedded to have forward compatible implementations.
	chat.UnimplementedChatServiceServer
}

func NewChatServer(store *Store) *ChatServer {
	return &ChatServer{
		store: store,
	}
}

func (s *ChatServer) CreateRoom(_ context.Context, request *chat.CreateRoomRequest) (*chat.CreateRoomResponse, error) {
	room, err := s.store.CreateRoom(request.UserId, request.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	slog.Info("room created", "room_id", room.ID)

	return &chat.CreateRoomResponse{
		RoomId: room.ID,
	}, nil
}

func (s *ChatServer) Connect(stream chat.ChatService_ConnectServer) error {
	in, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive message: %w", err)
	}

	connectRoom, ok := in.Payload.(*chat.ConnectRequest_ConnectRoom_)
	if !ok {
		return fmt.Errorf("failed to receive message: %w", err)
	}

	slog.Info("connect", "room_id", connectRoom.ConnectRoom.RoomId, "user_id", connectRoom.ConnectRoom.UserId)

	hub, err := s.store.GetRoomHub(connectRoom.ConnectRoom.RoomId)
	if err != nil {
		return fmt.Errorf("failed to get room hub: %w", err)
	}

	connection := hub.Connect(connectRoom.ConnectRoom.UserId, connectRoom.ConnectRoom.LastReadMessageId)
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
				slog.Error("failed to send message", "error", err)
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
			if err = hub.ReceiveMessage(msg); err != nil {
				return fmt.Errorf("failed to receive message: %w", err)
			}
		default:
			return fmt.Errorf("failed to receive message: %w", err)
		}
	}
}
