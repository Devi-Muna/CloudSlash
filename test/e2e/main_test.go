//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"testing"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

var (
	awsCfg          aws.Config
	DockerClient    *client.Client
	containerID     string
)

const (
	localstackImage = "localstack/localstack:3.0.2"
)

func getDockerSocket() string {
	if env := os.Getenv("E2E_DOCKER_SOCKET"); env != "" {
		return env
	}
	
	// 1. Try Standard Linux/Mac Docker
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "unix:///var/run/docker.sock"
	}
	
	// 2. Try OrbStack (Common on macOS)
	home, _ := os.UserHomeDir()
	orbPath := filepath.Join(home, ".orbstack/run/docker.sock")
	if _, err := os.Stat(orbPath); err == nil {
		return "unix://" + orbPath
	}
	
	// 3. Fallback
	return "unix:///var/run/docker.sock"
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. Setup Docker Client
	socket := getDockerSocket()

	cli, err := client.NewClientWithOpts(
		client.WithHost(socket),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		fmt.Printf("Failed to create docker client: %v\n", err)
		os.Exit(1)
	}

	DockerClient = cli


	// 2. Start LocalStack
	// Pull Image
	reader, err := cli.ImagePull(ctx, localstackImage, image.PullOptions{})
	if err != nil {
		fmt.Printf("Failed to pull image: %v\n", err)
		os.Exit(1)
	}
	io.Copy(io.Discard, reader) 

	// Create Container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: localstackImage,
		ExposedPorts: nat.PortSet{
			"4566/tcp": struct{}{},
		},
		Env: []string{"SERVICES=s3,ec2,sts,iam,pricing"}, // Optimize startup
	}, &container.HostConfig{
		AutoRemove: true,
		PortBindings: nat.PortMap{
			"4566/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Random port
				},
			},
		},
	}, nil, nil, "")
	if err != nil {
		fmt.Printf("Failed to create container: %v\n", err)
		os.Exit(1)
	}
	containerID = resp.ID

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		fmt.Printf("Failed to start container: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Started LocalStack container: %s\n", containerID)

	// Get Mapped Port
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		fmt.Printf("Failed to inspect container: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	
	bindings := inspect.NetworkSettings.Ports["4566/tcp"]
	if len(bindings) == 0 {
		fmt.Printf("No port binding found for 4566/tcp\n")
		cleanup()
		os.Exit(1)
	}
	port := bindings[0].HostPort
	endpointURL := fmt.Sprintf("http://localhost:%s", port)
	fmt.Printf("LocalStack mapped to %s\n", endpointURL)

	// Wait for Health
	waitForLocalStack(endpointURL)

	// 3. Configure SDK
	os.Setenv("AWS_ENDPOINT_URL", endpointURL)
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_REGION", "us-east-1")

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
			}, nil
		})),
		config.WithBaseEndpoint(endpointURL),
	)
	if err != nil {
		fmt.Printf("Failed to load AWS config: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	awsCfg = cfg

	// 4. Run Tests
	code := m.Run()

	// 5. Cleanup
	cleanup()
	os.Exit(code)
}

func cleanup() {
	if DockerClient != nil && containerID != "" {
		ctx := context.Background()
		// Force remove
		DockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
	}
}

func waitForLocalStack(endpoint string) {
	fmt.Println("Waiting for LocalStack...")
	client := &http.Client{Timeout: 1 * time.Second}
	for i := 0; i < 30; i++ {
		resp, err := client.Get(endpoint + "/_localstack/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				fmt.Println("LocalStack is ready!")
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Println("Timeout waiting for LocalStack")
}
