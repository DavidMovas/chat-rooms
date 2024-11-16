package server

import "time"

type Message struct {
	ID        string
	RoomID    string
	UserID    string
	Text      string
	CreatedAt time.Time
}
