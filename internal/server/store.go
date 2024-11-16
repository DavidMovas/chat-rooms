package server

import (
	"fmt"
	"github.com/google/uuid"
	"sync"
)

type Store struct {
	rooms   map[string]*Room
	roomsMx sync.RWMutex

	roomHub   map[string]*RoomHub
	roomHubMx sync.RWMutex

	messages   map[string][]*Message
	messagesMx sync.RWMutex
}

func NewStorage() *Store {
	return &Store{
		rooms:    make(map[string]*Room),
		roomHub:  make(map[string]*RoomHub),
		messages: make(map[string][]*Message),
	}
}

func (s *Store) CreateRoom(userID string, name string) (*Room, error) {
	room := &Room{
		ID:      uuid.New().String(),
		OwnerID: userID,
		Name:    name,
	}

	s.addRoom(room)

	return room, nil
}

func (s *Store) GetRoomHub(roomID string) (*RoomHub, error) {
	room := s.getRoom(roomID)
	if room == nil {
		return nil, fmt.Errorf("room not found")
	}

	return s.getOrCreateRoomHub(room)
}

func (s *Store) getRoom(roomID string) *Room {
	s.roomsMx.RLock()
	defer s.roomsMx.RUnlock()

	return s.rooms[roomID]
}

func (s *Store) addRoom(room *Room) {
	s.roomsMx.Lock()
	defer s.roomsMx.Unlock()

	s.rooms[room.ID] = room
}

func (s *Store) getOrCreateRoomHub(room *Room) (*RoomHub, error) {
	s.roomHubMx.Lock()
	hub := s.roomHub[room.ID]
	s.roomHubMx.RUnlock()

	if hub != nil {
		return hub, nil
	}

	s.roomHubMx.Lock()
	defer s.roomHubMx.Unlock()

	hub = s.roomHub[room.ID]
	if hub != nil {
		return hub, nil
	}

	hub, err := NewRoomHub(room, s)
	if err != nil {
		return nil, fmt.Errorf("failed to create room hub: %w", err)
	}

	s.roomHub[room.ID] = hub

	return hub, nil
}

func (s *Store) loadMessages(roomID string) ([]*Message, error) {
	s.messagesMx.RLock()
	defer s.messagesMx.RUnlock()

	return s.messages[roomID], nil
}

func (s *Store) saveMessage(message *Message) error {
	s.messagesMx.Lock()
	defer s.messagesMx.Unlock()

	s.messages[message.RoomID] = append(s.messages[message.RoomID], message)

	return nil
}

type Room struct {
	ID      string
	OwnerID string
	Name    string
}

type RoomHub struct {
	room         *Room
	store        *Store
	mx           sync.RWMutex
	messages     []*Message
	userChannels map[string]chan *Message
}

func NewRoomHub(room *Room, store *Store) (*RoomHub, error) {
	messages, err := store.loadMessages(room.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}

	return &RoomHub{
		room:         room,
		store:        store,
		messages:     messages,
		userChannels: make(map[string]chan *Message),
	}, nil
}

func (h *RoomHub) Connect(userID string, lastReadMessageID string) *Connection {
	h.mx.Lock()
	defer h.mx.Unlock()

	connection := &Connection{
		UserID:     userID,
		RoomID:     h.room.ID,
		Unread:     h.getUnreadMessages(lastReadMessageID),
		MessagesCh: make(chan *Message, 4),
		Disconnect: func() {
			h.disconnect(userID)
		},
	}

	h.userChannels[userID] = connection.MessagesCh

	return connection
}

func (h *RoomHub) ReceiveMessage(message *Message) error {
	if err := h.store.saveMessage(message); err != nil {
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

func (h *RoomHub) getUnreadMessages(lastReadMessageID string) []*Message {
	if lastReadMessageID == "" {
		return h.messages
	}

	i := len(h.messages) - 1
	for ; i >= 0 && h.messages[i].ID > lastReadMessageID; i-- {
		// empty
	}

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
