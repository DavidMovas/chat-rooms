package main

import (
	"context"
	"fmt"
	"github.com/DavidMovas/chat-rooms/apis/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func main() {
	c, err := grpc.NewClient("localhost:50000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	client := chat.NewChatServiceClient(c)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	res, err := client.CreateRoom(ctx, &chat.CreateRoomRequest{
		UserId: "1",
		Name:   "test",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(res.RoomId)
}
