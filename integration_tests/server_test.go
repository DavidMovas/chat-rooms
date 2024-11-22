package tests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DavidMovas/chat-rooms/apis/chat"
	"github.com/DavidMovas/chat-rooms/internal/config"
	"github.com/DavidMovas/chat-rooms/internal/server"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestServer(t *testing.T) {
	prepareInfrastructure(t, runServer)
}

func runServer(t *testing.T, redisConn string) {
	cfg := &config.Config{
		Port:     0,
		Local:    true,
		LogLevel: "info",
		RedisURL: redisConn,
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	if err := rdb.Ping(shortCallCtx()).Err(); err != nil {
		require.NoError(t, err)
	}

	defer func() {
		_ = rdb.Close()
	}()

	srv, err := server.NewServer(cfg, rdb)
	require.NoError(t, err)

	go func() {
		if err = srv.Start(); err != nil {
			require.NoError(t, err)
		}
	}()

	var port int
	retry.Run(t, func(r *retry.R) {
		port, err = srv.Port()
		if err != nil {
			require.NoError(r, err)
		}
	})

	tests(t, port)
}

func tests(t *testing.T, port int) {
	t.Logf("starting integration_tests on port %d", port)

	addr := fmt.Sprintf("localhost:%d", port)

	var clients []*RoomClient
	for i := 0; i < 10; i++ {
		userID := fmt.Sprintf("user-%d", i)
		clients = append(clients, createClient(t, addr, userID))
	}

	t.Log("clients created")

	creator := clients[0]

	roomID, err := creator.CreateRoom(shortCallCtx(), "room-1")
	require.NoError(t, err)
	t.Logf("room created: %s", roomID)

	testCtx, testCancel := context.WithCancel(context.Background())
	connections, connectionCtx := errgroup.WithContext(testCtx)

	for _, client := range clients {
		client := client
		connections.Go(func() error {
			return client.Connect(connectionCtx, roomID)
		})
	}
	t.Log("all clients initiated connection")

	for _, client := range clients {
		client.WaitConnected()
	}
	t.Log("all clients connected")

	messagesCount := len(clients) * 100
	for i := 0; i < messagesCount; i++ {
		clientM, messageN := i%len(clients), i/len(clients)

		text := fmt.Sprintf("message-%d-%d", messageN, clientM)
		sendErr := clients[clientM].SendMessage(text)
		require.NoError(t, sendErr)
	}
	t.Log("all clients sent messages")

	time.Sleep(time.Second)

	testCancel()
	if err = connections.Wait(); !isCancelled(err) {
		require.NoError(t, err)
	}
}

type RoomClient struct {
	client    chat.ChatServiceClient
	stream    chat.ChatService_ConnectClient
	userID    string
	connected chan struct{}

	messages   []*chat.Message
	messagesMx sync.RWMutex
}

func createClient(t *testing.T, addr string, userID string) *RoomClient {
	c, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := chat.NewChatServiceClient(c)
	return NewRoomClient(client, userID)
}

func NewRoomClient(client chat.ChatServiceClient, userID string) *RoomClient {
	return &RoomClient{
		client:    client,
		userID:    userID,
		connected: make(chan struct{}),
	}
}

func (c *RoomClient) CreateRoom(ctx context.Context, name string) (string, error) {
	res, err := c.client.CreateRoom(ctx, &chat.CreateRoomRequest{
		UserId: c.userID,
		Name:   name,
	})
	if err != nil {
		return "", err
	}

	return res.RoomId, nil
}

func (c *RoomClient) Connect(ctx context.Context, roomID string) error {
	stream, err := c.client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	err = stream.Send(&chat.ConnectRequest{
		Payload: &chat.ConnectRequest_ConnectRoom_{
			ConnectRoom: &chat.ConnectRequest_ConnectRoom{
				RoomId:                roomID,
				UserId:                c.userID,
				LastReadMessageNumber: -1,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.stream = stream
	close(c.connected)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var res *chat.ConnectResponse
		res, err = stream.Recv()
		if err != nil {
			return fmt.Errorf("failed to receive message: %w", err)
		}

		switch p := res.Payload.(type) {
		case *chat.ConnectResponse_Message:
			c.addMessages(p.Message)
		case *chat.ConnectResponse_MessageList:
			c.addMessages(p.MessageList.Messages...)
		}
	}
}

func (c *RoomClient) WaitConnected() {
	<-c.connected
}

func (c *RoomClient) SendMessage(text string) error {
	return c.stream.Send(&chat.ConnectRequest{
		Payload: &chat.ConnectRequest_SendMessage_{
			SendMessage: &chat.ConnectRequest_SendMessage{
				Text: text,
			},
		},
	})
}

func (c *RoomClient) Messages() []*chat.Message {
	c.messagesMx.RLock()
	defer c.messagesMx.RUnlock()

	return c.messages
}

func (c *RoomClient) UserID() string {
	return c.userID
}

func (c *RoomClient) addMessages(messages ...*chat.Message) {
	c.messagesMx.Lock()
	defer c.messagesMx.Unlock()

	c.messages = append(c.messages, messages...)
}

func shortCallCtx() context.Context {
	ctx, f := context.WithTimeout(context.Background(), time.Second)
	_ = f
	return ctx
}

func isCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled
}
