package testhelpers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	ggcrauthn "github.com/google/go-containerregistry/pkg/authn"
	"golang.org/x/crypto/bcrypt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

var registryBaseImage = "micahyoung/registry:2"

type DockerRegistry struct {
	Name     string
	Host     string
	Port     string
	Username string
	Password string
}

type testKeychain struct {
	resolveFunc func(resource ggcrauthn.Resource) (ggcrauthn.Authenticator, error)
}

func (t testKeychain) Resolve(resource ggcrauthn.Resource) (ggcrauthn.Authenticator, error) {
	return t.resolveFunc(resource)
}

func NewDockerRegistry() *DockerRegistry {
	return &DockerRegistry{
		Name:     "test-registry-" + RandString(10),
		Username: RandString(10),
		Password: RandString(10),
	}
}

func (registry *DockerRegistry) Start(t *testing.T) {
	t.Log("run registry")
	t.Helper()

	htpasswdTar := generateHtpasswd(t, registry.Username, registry.Password)

	AssertNil(t, PullImage(DockerCli(t), registryBaseImage, ""))
	ctx := context.Background()
	ctr, err := DockerCli(t).ContainerCreate(ctx, &container.Config{
		Image: registryBaseImage,
		Env: []string{
			"REGISTRY_STORAGE_DELETE_ENABLED=true",
			"REGISTRY_AUTH=htpasswd",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm",
			"REGISTRY_AUTH_HTPASSWD_PATH=/registry_test_htpasswd",
		},
	}, &container.HostConfig{
		AutoRemove: true,
		PortBindings: nat.PortMap{
			"5000/tcp": []nat.PortBinding{{}},
		},
	}, nil, registry.Name)
	AssertNil(t, err)
	err = DockerCli(t).CopyToContainer(ctx, ctr.ID, "/", htpasswdTar, types.CopyToContainerOptions{})
	AssertNil(t, err)

	err = DockerCli(t).ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
	AssertNil(t, err)

	inspect, err := DockerCli(t).ContainerInspect(ctx, ctr.ID)
	AssertNil(t, err)
	registry.Port = inspect.NetworkSettings.Ports["5000/tcp"][0].HostPort

	registry.Host, err = getRegistryHostname()
	AssertNil(t, err)

	Eventually(t, func() bool {
		txt, err := HttpGetAuthE(fmt.Sprintf("http://%s:%s/v2/", registry.Host, registry.Port), registry.Username, registry.Password)
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

func (registry *DockerRegistry) GGCRKeychain() ggcrauthn.Keychain {
	return testKeychain{

		func(resource ggcrauthn.Resource) (ggcrauthn.Authenticator, error) {
			//matches localhost:32918 or localhost:32918/pack-image-test-uhiphgyvol
			if strings.HasPrefix(resource.String(), registry.Host+":"+registry.Port) {
				return &ggcrauthn.Basic{Username: registry.Username, Password: registry.Password}, nil
			}
			return ggcrauthn.Anonymous, nil
		},
	}
}

func (registry *DockerRegistry) DockerRegistryAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"username":"%s","password":"%s"}`, registry.Username, registry.Password)))
}

func generateHtpasswd(t *testing.T, username string, password string) io.Reader {
	//https://docs.docker.com/registry/deploying/#restricting-access
	//https://github.com/foomo/htpasswd/blob/e3a90e78da9cff06a83a78861847aa9092cbebdd/hashing.go#L23
	passwordBytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	reader, err := CreateSingleFileTar("/registry_test_htpasswd", username+":"+string(passwordBytes), "linux")
	AssertNil(t, err)

	return reader
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
