package main

import (
	"context"
	"fmt"
	"github.com/DavidMovas/chat-rooms/apis/chat"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"math/rand"
	"time"
)

func main() {
	c, err := grpc.NewClient("localhost:50000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	client := chat.NewChatServiceClient(c)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	user := createUser()

	roomName := "Room"
	res, err := client.CreateRoom(ctx, &chat.CreateRoomRequest{
		UserId: user.id,
		Name:   roomName,
	})

	fmt.Printf("User Name: %s\n Connected to Room ID: %s\n", user.name, res.RoomId)

	stream, err := client.Connect(ctx)

	_ = stream.Send(&chat.ConnectRequest{
		Payload: &chat.ConnectRequest_ConnectRoom_{
			ConnectRoom: &chat.ConnectRequest_ConnectRoom{
				RoomId:            res.RoomId,
				UserId:            user.id,
				LastReadMessageId: roomName,
			},
		},
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := stream.Recv()
			if err != nil {
				panic(err)
			}

			switch p := resp.Payload.(type) {
			case *chat.ConnectResponse_MessageList:
				for _, msg := range p.MessageList.Messages {
					fmt.Printf("%s: %s\n", msg.UserId, msg.Text)
				}
			case *chat.ConnectResponse_Message:
				fmt.Printf("%s: %s\n", p.Message.UserId, p.Message.Text)
			}

			time.Sleep(time.Second * 1)

			text := fmt.Sprintf("%s says hello", user.name)
			fmt.Printf("%s: %s\n", user.id, text)

			err = stream.Send(&chat.ConnectRequest{
				Payload: &chat.ConnectRequest_SendMessage_{
					SendMessage: &chat.ConnectRequest_SendMessage{
						Text: text,
					},
				},
			})

			if err != nil && err != io.EOF {
				panic(err)
			}
		}
	}
}

type user struct {
	id   string
	name string
}

func createUser() *user {
	id, _ := uuid.NewV6()
	return &user{
		id:   id.String(),
		name: fmt.Sprintf("User %d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10)),
	}
}
