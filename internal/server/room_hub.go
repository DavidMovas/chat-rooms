package server

import (
	"context"
	"fmt"
	"sync"
)

type RoomHub struct {
	room  *Room
	store *Store
	mx    sync.RWMutex

	lastNumber   int
	messagesMx   sync.RWMutex
	messages     []*Message
	userChannels map[string]chan *Message
}

func newRoomHub(ctx context.Context, room *Room, store *Store) (*RoomHub, error) {
	messages, lastNumber, err := store.LoadMessages(ctx, room.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}

	return &RoomHub{
		room:         room,
		store:        store,
		lastNumber:   lastNumber,
		messages:     messages,
		userChannels: make(map[string]chan *Message),
	}, nil
}

func (h *RoomHub) Connect(userID string, lastReadMessageNumber int64) *Connection {
	h.mx.Lock()
	defer h.mx.Unlock()

	connection := &Connection{
		UserID:     userID,
		RoomID:     h.room.ID,
		Unread:     h.getUnreadMessages(lastReadMessageNumber),
		MessagesCh: make(chan *Message, 4),
		Disconnect: func() {
			h.disconnect(userID)
		},
	}

	h.userChannels[userID] = connection.MessagesCh

	return connection
}

func (h *RoomHub) ReceiveMessage(ctx context.Context, message *Message) error {
	if err := h.saveMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	h.mx.RLock()
	defer h.mx.RUnlock()

	h.messages = append(h.messages, message)

	for _, ch := range h.userChannels {
		ch <- message
	}

	return nil
}

func (h *RoomHub) saveMessage(ctx context.Context, message *Message) error {
	h.messagesMx.Lock()
	defer h.messagesMx.Unlock()

	message.Number = h.lastNumber + 1
	if err := h.store.SaveMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	h.messages = append(h.messages, message)
	h.lastNumber++

	return nil
}

func (h *RoomHub) getUnreadMessages(lastReadMessageNumber int64) []*Message {
	if lastReadMessageNumber <= -1 {
		return h.messages
	}

	i := len(h.messages) - 1

	return h.messages[i+1:]
}

func (h *RoomHub) disconnect(userID string) {
	h.mx.Lock()
	defer h.mx.Unlock()

	ch := h.userChannels[userID]
	if ch != nil {
		close(ch)
		delete(h.userChannels, userID)
	}
}

type Connection struct {
	UserID     string
	RoomID     string
	Unread     []*Message
	MessagesCh chan *Message
	Disconnect func()
}
