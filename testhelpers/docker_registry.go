package testhelpers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

var registryBaseImage = "micahyoung/registry:2"

type DockerRegistry struct {
	Host              string
	Port              string
	Name              string
}

func NewDockerRegistry() *DockerRegistry {
	return &DockerRegistry{}
}

func (registry *DockerRegistry) Start(t *testing.T) {
	t.Log("run registry")
	t.Helper()
	registry.Name = "test-registry-" + RandString(10)

	AssertNil(t, PullImage(DockerCli(t), registryBaseImage))
	ctx := context.Background()
	ctr, err := DockerCli(t).ContainerCreate(ctx, &container.Config{
		Image: registryBaseImage,
		Env:   []string{"REGISTRY_STORAGE_DELETE_ENABLED=true"},
	}, &container.HostConfig{
		AutoRemove: true,
		PortBindings: nat.PortMap{
			"5000/tcp": []nat.PortBinding{{}},
		},
	}, nil, registry.Name)
	AssertNil(t, err)
	err = DockerCli(t).ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
	AssertNil(t, err)

	inspect, err := DockerCli(t).ContainerInspect(ctx, ctr.ID)
	AssertNil(t, err)
	registry.Port = inspect.NetworkSettings.Ports["5000/tcp"][0].HostPort

	registry.Host, err = getRegistryHostname()
	AssertNil(t, err)

	Eventually(t, func() bool {
		txt, err := HttpGetE(fmt.Sprintf("http://%s:%s/v2/", registry.Host, registry.Port))
		return err == nil && txt != ""
	}, 100*time.Millisecond, 10*time.Second)
}

func (registry *DockerRegistry) Stop(t *testing.T) {
	t.Log("stop registry")
	t.Helper()
	if registry.Name != "" {
		DockerCli(t).ContainerKill(context.Background(), registry.Name, "SIGKILL")
		DockerCli(t).ContainerRemove(context.TODO(), registry.Name, types.ContainerRemoveOptions{Force: true})
	}
}

func getRegistryHostname() (string, error) {
	dockerHost := os.Getenv(("DOCKER_HOST"))
	if dockerHost != "" {
		url, err := url.Parse(dockerHost)
		if err != nil {
			return "", err
		}
		return url.Hostname(), nil
	}
	return "localhost", nil
}
