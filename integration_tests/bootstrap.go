package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	redisContainerName  = "redis"
	redisContainerPort  = "6379/tcp"
	redisContainerImage = "redis"
)

func prepareInfrastructure(t *testing.T, runServerFunc func(t *testing.T, redisConn string)) {
	// Redis container setup
	ctx := context.Background()
	redisContainer, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         redisContainerName,
			Image:        redisContainerImage,
			ExposedPorts: []string{redisContainerPort},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})

	require.NoError(t, err)

	redisPort, err := redisContainer.MappedPort(ctx, redisContainerPort)
	require.NoError(t, err)

	redisConn := fmt.Sprintf("localhost:%d", redisPort.Int())
	_ = redisConn

	defer cleanUp(ctx, t, redisContainer.Terminate)

	runServerFunc(t, redisConn)
}

func cleanUp(ctx context.Context, t *testing.T, termsFunc ...func(context.Context) error) {
	for _, f := range termsFunc {
		require.NoError(t, f(ctx))
	}
}
