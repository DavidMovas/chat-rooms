package server

import (
	"encoding"
	"encoding/json"
	"time"
)

var _ encoding.BinaryUnmarshaler = (*Message)(nil)
var _ encoding.BinaryMarshaler = (*Message)(nil)

type Message struct {
	Number    int
	RoomID    string
	UserID    string
	Text      string
	CreatedAt time.Time
}

func (m *Message) MarshalBinary() (data []byte, err error) {
	return json.Marshal(m)
}

func (m *Message) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, m)
}
