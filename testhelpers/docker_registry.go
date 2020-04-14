package testhelpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

type DockerRegistry struct {
	Port string
	Name string
}

var registryImageNames = map[string]string{
	"linux":   "registry:2",
	"windows": "stefanscherer/registry-windows:2.6.2",
}

func NewDockerRegistry() *DockerRegistry {
	return &DockerRegistry{}
}

func (registry *DockerRegistry) Start(t *testing.T) {
	t.Log("run registry")
	t.Helper()
	registry.Name = "test-registry-" + RandString(10)

	ctx := context.Background()
	daemonInfo, err := DockerCli(t).Info(ctx)
	AssertNil(t, err)

	registryImageName := registryImageNames[daemonInfo.OSType]
	AssertNil(t, PullImage(DockerCli(t), registryImageName))

	ctr, err := DockerCli(t).ContainerCreate(ctx, &container.Config{
		Image: registryImageName,
		Env:   []string{"REGISTRY_STORAGE_DELETE_ENABLED=true"},
	}, &container.HostConfig{
		AutoRemove: true,
		PortBindings: nat.PortMap{
			"5000/tcp": []nat.PortBinding{{}},
		},
		Isolation: fastestIsolation(DockerCli(t)),
	}, nil, registry.Name)
	AssertNil(t, err)
	err = DockerCli(t).ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
	AssertNil(t, err)

	inspect, err := DockerCli(t).ContainerInspect(ctx, ctr.ID)
	AssertNil(t, err)
	registry.Port = inspect.NetworkSettings.Ports["5000/tcp"][0].HostPort

	Eventually(t, func() bool {
		txt, err := HTTPGetE(fmt.Sprintf("http://localhost:%s/v2/", registry.Port))
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
