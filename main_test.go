package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"testing"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCastError(t *testing.T) {
	require := require.New(t)
	t.Run("use as", func(t *testing.T) {

		_, port, _ := SetupESDBcontainerWithEnvironment(t, context.Background(), nil, DefaultESDBcontainerEnv)

		config, err := esdb.ParseConnectionString("esdb://localhost:" + port.Port() + "?tls=false")
		require.NoError(err)

		client, err := esdb.NewClient(config)
		require.NoError(err)

		stream, err := client.ReadStream(context.Background(), "somestream", esdb.ReadStreamOptions{}, 1)
		require.NoError(err)

		// This corresponds to your docs at:
		// https://developers.eventstore.com/clients/grpc/reading-events.html#checking-if-the-stream-exists
		for {
			event, err := stream.Recv()

			var typedError *esdb.ResourceNotFoundError

			// This is somewhat confusing, but it NEEDS to be the pointer to what implements the error interface
			// (in this case the error interface is implemented on the pointer receiver), so it's the address to the pointer
			if ok := errors.As(err, &typedError); ok {
				fmt.Print("Stream not found")
				break
			} else if errors.Is(err, io.EOF) {
				fmt.Print("Stream read to the end")
				break
			} else {
				t.Fail()
			}

			fmt.Printf("Event> %v", event)
		}
	})
}

func SetupESDBcontainerWithEnvironment(t *testing.T, ctx context.Context, networkName *string,
	env map[string]string,
) (testcontainers.Container, nat.Port, string) {
	ESDBcontainerName := "EVENSTORE"

	req := testcontainers.ContainerRequest{
		Image:        GenerateDockerImageName(),
		ExposedPorts: []string{"2113/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForHTTP("/gossip").WithPort(nat.Port("2113/tcp")).WithStatusCodeMatcher(
				func(status int) bool { return status == 200 }),
			wait.ForHTTP("/stats").WithPort(nat.Port("2113/tcp")).WithStatusCodeMatcher(
				func(status int) bool { return status == 200 }),
		),
		Env:  env,
		Name: ESDBcontainerName,
	}

	container, containerErr := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            false,
		// Logger:           noLogger{},
	})

	if containerErr != nil {
		if containerErr != nil {
			t.Fatal("Container error: ", containerErr)
		}
	}

	// Get port
	mappedPort, mappedPortErr := container.MappedPort(ctx, "2113")
	if mappedPortErr != nil {
		t.Fatal("mappedPortErr error: ", mappedPortErr)
	}

	return container, mappedPort, ESDBcontainerName
}

func GenerateDockerImageName() string {
	// Get the current runtime architecture
	arch := runtime.GOARCH

	// Check if the architecture is ARMv8
	isARMv8 := arch == "arm64" || arch == "aarch64"
	var dockertag string
	if isARMv8 {
		dockertag = "24.2.0-alpha-arm64v8"
	} else {
		dockertag = "24.2.0-jammy"
	}
	return "eventstore/eventstore:" + dockertag
}

var DefaultESDBcontainerEnv = map[string]string{
	"EVENTSTORE_CLUSTER_SIZE":               "1",
	"EVENTSTORE_RUN_PROJECTIONS":            "All",
	"EVENTSTORE_START_STANDARD_PROJECTIONS": "true",
	"EVENTSTORE_ENABLE_ATOM_PUB_OVER_HTTP":  "true",
	"EVENTSTORE_HTTP_PORT":                  "2113",
	"EVENTSTORE_INSECURE":                   "true",
}
