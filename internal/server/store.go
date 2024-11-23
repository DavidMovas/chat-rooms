package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DavidMovas/chat-rooms/internal/config"
	"github.com/google/uuid"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	rdb *redis.Client

	maxMessages  int
	maxRetention time.Duration

	roomHub   map[string]*RoomHub
	roomHubMx sync.RWMutex
}

func NewStorage(rdb *redis.Client, cfg *config.Config) *Store {
	return &Store{
		rdb:          rdb,
		roomHub:      make(map[string]*RoomHub),
		maxMessages:  cfg.MaxMessages,
		maxRetention: cfg.MaxRetention,
	}
}

func (s *Store) CreateRoom(ctx context.Context, userID string, name string) (*Room, error) {
	room := &Room{
		ID:      s.roomKey(),
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

	return s.getOrCreateRoomHub(ctx, room)
}

func (s *Store) LoadMessages(ctx context.Context, roomID string) (messages []*Message, lastNumber int, err error) {
	tx := s.rdb.TxPipeline()

	getMessagesCmd := tx.ZRevRangeByScore(ctx, s.roomMessagesKey(roomID), &redis.ZRangeBy{
		Count: int64(s.maxMessages),
		Min:   fmt.Sprintf("%d", time.Now().Add(-s.maxRetention).UnixNano()),
		Max:   "+inf",
	})

	getMessagesNumberCmd := tx.Get(ctx, s.messageNumberKey(roomID))

	if _, err = tx.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, 0, fmt.Errorf("failed to load messages: %w", err)
	}

	err = getMessagesCmd.ScanSlice(&messages)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load messages: %w", err)
	}

	lastNumber, err = getMessagesNumberCmd.Int()
	switch {
	case errors.Is(err, redis.Nil):
		lastNumber = -1
	case err != nil:
		return nil, 0, fmt.Errorf("failed to load messages: %w", err)
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Number < messages[j].Number
	})

	return messages, lastNumber, nil
}

func (s *Store) SaveMessage(ctx context.Context, message *Message) error {
	tx := s.rdb.TxPipeline()

	addMessageCmd := tx.ZAdd(ctx, s.roomMessagesKey(message.RoomID), redis.Z{
		Score:  float64(message.CreatedAt.UnixNano()),
		Member: message,
	})

	updateNumberCmd := tx.Set(ctx, s.messageNumberKey(message.RoomID), message.Number, 0)

	if _, err := tx.Exec(ctx); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	if err := addMessageCmd.Err(); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	if err := updateNumberCmd.Err(); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	return nil
}

func (s *Store) getOrCreateRoomHub(ctx context.Context, room *Room) (*RoomHub, error) {
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

	hub, err := newRoomHub(ctx, room, s)
	if err != nil {
		return nil, fmt.Errorf("failed to create room hub: %w", err)
	}

	s.roomHub[room.ID] = hub

	return hub, nil
}

func (s *Store) roomKey() string {
	return fmt.Sprintf("rooms:%s:data", uuid.New().String())
}

func (s *Store) roomMessagesKey(roomID string) string {
	return fmt.Sprintf("%s:messages", roomID)
}

func (s *Store) messageNumberKey(roomID string) string {
	return fmt.Sprintf("%s:last_message_number", roomID)
}

type Room struct {
	ID      string
	OwnerID string
	Name    string
}
