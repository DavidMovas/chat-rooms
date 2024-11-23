package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/DavidMovas/chat-rooms/apis/chat"
	"github.com/DavidMovas/chat-rooms/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	clientsAmount       = 15
	communicateDuration = 30
	offset              = 10_000
)

func main() {
	cfg := &config.Config{
		Port:     50000,
		Local:    true,
		LogLevel: "info",
		RedisURL: "localhost:6379",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*communicateDuration)
	defer cancel()

	var users []*user
	for i := 0; i < clientsAmount; i++ {
		c, err := grpc.NewClient(fmt.Sprintf("localhost:%d", cfg.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			panic(err)
		}

		randOffset := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(offset)
		ticker := time.NewTicker(time.Millisecond * time.Duration(randOffset))

		u := &user{
			id:        fmt.Sprintf("user-%d", i),
			name:      fmt.Sprintf("user-%d", i),
			client:    chat.NewChatServiceClient(c),
			connectCh: make(chan struct{}),
			cancelCh:  make(chan struct{}),
			ticker:    ticker,
		}
		users = append(users, u)
	}

	creator := users[0]
	roomID, err := creator.createRoom()
	if err != nil {
		panic(err)
	}

	fmt.Printf("room created: %s\n", roomID)

	connections, connectionCtx := errgroup.WithContext(ctx)
	for _, u := range users {
		u := u
		u.roomID = roomID
		connections.Go(func() error {
			return u.connect(connectionCtx)
		})
	}

	for _, u := range users {
		u.waitUntilConnect()
	}

	fmt.Printf("all users connected\n")
	fmt.Printf("communicating\n")

	for _, u := range users {
		go func(u *user) {
			if err := u.sendMessage(ctx); err != nil {
				if !isCancelled(err) {
					fmt.Printf("error sending message: %s\n", err.Error())
					panic(err)
				}
			}
		}(u)
	}

	for _, u := range users {
		u.waitUntilDisconnect()
	}

	fmt.Println("all users disconnected")
}

type user struct {
	id     string
	name   string
	roomID string

	connectCh chan struct{}
	cancelCh  chan struct{}

	sendMx sync.Mutex

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

func (u *user) connect(ctx context.Context) error {
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

	close(u.connectCh)

	closeSend := make(chan error, 1)
	go func() {
		<-ctx.Done()
		u.sendMx.Lock()
		defer u.sendMx.Unlock()
		closeSend <- u.stream.CloseSend()
	}()

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

func (u *user) waitUntilConnect() {
	<-u.connectCh
}

func (u *user) waitUntilDisconnect() {
	<-u.cancelCh
}

func (u *user) sendMessage(ctx context.Context) error {
	defer func() {
		u.ticker.Stop()
		close(u.cancelCh)
	}()

	for {
		select {
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
		case <-ctx.Done():
			return ctx.Err()
		case <-u.cancelCh:
			return nil
		}
	}
}

func isCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.Canceled || status.Code(err) == codes.DeadlineExceeded
}
