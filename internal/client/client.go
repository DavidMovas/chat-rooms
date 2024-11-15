package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/DavidMovas/chat-rooms/apis/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math/rand"
	"sync"
	"time"
)

func main() {
	addr := "localhost:50000"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	var users []*user
	for i := 0; i < 5; i++ {
		c, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			panic(err)
		}

		u := &user{
			id:     fmt.Sprintf("user-%d", i),
			name:   fmt.Sprintf("user-%d", i),
			client: chat.NewChatServiceClient(c),
		}
		users = append(users, u)
	}

	creator := users[0]
	roomID, err := creator.createRoom()
	if err != nil {
		panic(err)
	}

	fmt.Printf("room created: %s\n", roomID)

	w := sync.WaitGroup{}

	var errs []error
	for _, u := range users {
		w.Add(1)
		u.roomId = roomID

		go func(u *user) {
			if err := u.connect(ctx, &w); err != nil {
				errs = append(errs, err)
			}

			fmt.Printf("user %s connected\n", u.id)
		}(u)
	}

	w.Wait()
	fmt.Printf("all users connected\n")

	if len(errs) > 0 {
		panic(errors.Join(errs...))
	}

	fmt.Printf("communicating\n")

	wg := sync.WaitGroup{}

	for _, u := range users {
		wg.Add(1)
		go func(u *user) {
			if err := u.communicate(ctx, &wg); err != nil {
				if !errors.Is(err, context.Canceled) {
					panic(err)
				}
			}
		}(u)
	}

	wg.Wait()
}

type user struct {
	id     string
	name   string
	roomId string
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
				RoomId:            u.roomId,
				UserId:            u.id,
				LastReadMessageId: "",
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
	offset := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(5000)
	u.ticker = time.NewTicker(time.Millisecond * time.Duration(offset))

	defer func() {
		u.ticker.Stop()
		wg.Done()
	}()

	go func() {
		err = u.sendMessage(ctx)
	}()

	go func() {
		err = u.receiveMessage(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			return err
		}
	}
}

func (u *user) sendMessage(ctx context.Context) error {
	var err error

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		select {
		case <-u.ticker.C:
			err = u.stream.Send(&chat.ConnectRequest{
				Payload: &chat.ConnectRequest_SendMessage_{
					SendMessage: &chat.ConnectRequest_SendMessage{
						Text: "Hello world!",
					},
				},
			})
			if err != nil {
				return err
			}
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
