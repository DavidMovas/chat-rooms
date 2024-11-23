package main

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/DavidMovas/chat-rooms/apis/chat"
	"github.com/DavidMovas/chat-rooms/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const clientsAmount = 15

func main() {
	cfg := &config.Config{
		Port:     50000,
		Local:    true,
		LogLevel: "info",
		RedisURL: "localhost:6379",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	var users []*user
	for i := 0; i < clientsAmount; i++ {
		c, err := grpc.NewClient(fmt.Sprintf("localhost:%d", cfg.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			panic(err)
		}

		offset1 := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10_000)
		offset2 := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10_000)
		ticker := time.NewTicker(time.Millisecond * time.Duration((offset1+offset2)/2))

		u := &user{
			id:       fmt.Sprintf("user-%d", i),
			name:     fmt.Sprintf("user-%d", i),
			client:   chat.NewChatServiceClient(c),
			canselCh: make(chan struct{}),
			ticker:   ticker,
		}
		users = append(users, u)
	}

	creator := users[0]
	roomID, err := creator.createRoom()
	if err != nil {
		panic(err)
	}

	//roomID := "rooms:ID:info"

	fmt.Printf("room created: %s\n", roomID)

	var wg sync.WaitGroup
	var errs []error

	for _, u := range users {
		wg.Add(1)
		u.roomID = roomID
		go func(u *user) {
			if err := u.connect(ctx, &wg); err != nil {
				errs = append(errs, err)
			}

			fmt.Printf("user %s connected\n", u.id)
		}(u)
	}

	wg.Wait()
	fmt.Printf("all users connected\n")

	if len(errs) > 0 {
		panic(errors.Join(errs...))
	}

	fmt.Printf("communicating\n")

	wg = sync.WaitGroup{}

	for _, u := range users {
		wg.Add(1)
		go func(u *user) {
			if err := u.communicate(ctx, &wg); err != nil {
				if status.Code(err) != codes.Canceled || err != io.EOF {
					fmt.Printf("error: %s\n", err)
					panic(err)
				}
			}
		}(u)
	}

	wg.Wait()

	fmt.Println("all users disconnected")
}

type user struct {
	id     string
	name   string
	roomID string

	canselCh chan struct{}

	client chat.ChatServiceClient
	stream chat.ChatService_ConnectClient

	ticker *time.Ticker
}

func (u *user) createRoom() (string, error) {
	res, err := u.client.CreateRoom(context.Background(), &chat.CreateRoomRequest{
		UserId: u.id,
		Name:   "room-1",
	})
	if err != nil {
		return "", err
	}

	return res.RoomId, err
}

func (u *user) connect(ctx context.Context, w *sync.WaitGroup) error {
	defer w.Done()
	stream, err := u.client.Connect(ctx)
	if err != nil {
		return err
	}

	u.stream = stream

	err = u.stream.Send(&chat.ConnectRequest{
		Payload: &chat.ConnectRequest_ConnectRoom_{
			ConnectRoom: &chat.ConnectRequest_ConnectRoom{
				RoomId:                u.roomID,
				UserId:                u.id,
				LastReadMessageNumber: -1,
			},
		},
	})
	if err != nil {
		return err
	}

	w.Done()

	for {
		if ctx.Err() != nil {
			return err
		}

		var res *chat.ConnectResponse
		res, err = u.stream.Recv()
		if err != nil {
			return err
		}

		switch p := res.Payload.(type) {
		case *chat.ConnectResponse_MessageList:
			for _, m := range p.MessageList.Messages {
				fmt.Printf("User: %s\n Read from user: %s\n Message: %s\n\n", u.id, m.UserId, m.Text)
			}
		case *chat.ConnectResponse_Message:
			fmt.Printf("User: %s\n Read from user: %s\n Message: %s\n\n", u.id, p.Message.UserId, p.Message.Text)
		}
	}
}

func (u *user) communicate(ctx context.Context, wg *sync.WaitGroup) error {
	var err error

	defer func() {
		u.ticker.Stop()
		close(u.canselCh)
		wg.Done()
	}()

	go func() {
		err = u.sendMessage(ctx)
	}()

	go func() {
		err = u.receiveMessage(ctx)
	}()

	<-ctx.Done()
	return err
}

func (u *user) sendMessage(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-u.ticker.C:
			err := u.stream.Send(&chat.ConnectRequest{
				Payload: &chat.ConnectRequest_SendMessage_{
					SendMessage: &chat.ConnectRequest_SendMessage{
						Text: "Hello world!",
					},
				},
			})
			if err != nil {
				return err
			}
		case <-u.canselCh:
			return nil
		}
	}
}

func (u *user) receiveMessage(ctx context.Context) error {
	var err error

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var res *chat.ConnectResponse
		res, err = u.stream.Recv()
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-u.canselCh:
			return nil
		default:
		}

		switch p := res.Payload.(type) {
		case *chat.ConnectResponse_MessageList:
			for _, m := range p.MessageList.Messages {
				fmt.Printf("User: %s\n Read from user: %s\n Message: %s\n\n", u.id, m.UserId, m.Text)
			}
		case *chat.ConnectResponse_Message:
			fmt.Printf("User: %s\n Read from user: %s\n Message: %s\n\n", u.id, p.Message.UserId, p.Message.Text)
		}
	}
}
