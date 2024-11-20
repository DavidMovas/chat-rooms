package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"sync"
)

type Store struct {
	rdb *redis.Client

	roomHub   map[string]*RoomHub
	roomHubMx sync.RWMutex

	messages   map[string][]*Message
	messagesMx sync.RWMutex
}

func NewStorage(rdb *redis.Client) *Store {
	return &Store{
		rdb:      rdb,
		roomHub:  make(map[string]*RoomHub),
		messages: make(map[string][]*Message),
	}
}

func (s *Store) CreateRoom(ctx context.Context, userID string, name string) (*Room, error) {
	room := &Room{
		ID:      fmt.Sprintf("room:%s:info", uuid.New().String()),
		OwnerID: userID,
		Name:    name,
	}

	bytes, err := json.Marshal(room)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal room: %w", err)
	}

	err = s.rdb.Set(ctx, room.ID, string(bytes), 0).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	return room, nil
}

func (s *Store) GetRoomHub(ctx context.Context, roomID string) (*RoomHub, error) {
	res, err := s.rdb.Get(ctx, roomID).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	var room *Room
	if err = json.Unmarshal([]byte(res), &room); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room: %w", err)
	}

	return s.getOrCreateRoomHub(room)
}

func (s *Store) getOrCreateRoomHub(room *Room) (*RoomHub, error) {
	s.roomHubMx.RLock()
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
