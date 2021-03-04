package testhelpers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"golang.org/x/crypto/bcrypt"
)

type DockerRegistry struct {
	Host            string
	Port            string
	Name            string
	DockerDirectory string
	username        string
	password        string
}

var registryImageName = "micahyoung/registry:latest"

func NewDockerRegistry() *DockerRegistry {
	return &DockerRegistry{
		Name: "test-registry-" + RandString(10),
	}
}

func NewDockerRegistryWithAuth(dockerConfigDir string) *DockerRegistry {
	return &DockerRegistry{
		Name:            "test-registry-" + RandString(10),
		username:        RandString(10),
		password:        RandString(10),
		DockerDirectory: dockerConfigDir,
	}
}

func (r *DockerRegistry) Start(t *testing.T) {
	r.Host = DockerHostname(t)

	t.Logf("run registry on %s", r.Host)
	t.Helper()

	PullIfMissing(t, DockerCli(t), registryImageName)

	var htpasswdTar io.ReadCloser
	registryEnv := []string{"REGISTRY_STORAGE_DELETE_ENABLED=true"}
	if r.username != "" {
		// Create htpasswdTar and configure registry env
		tempDir, err := ioutil.TempDir("", "test.registry")
		AssertNil(t, err)
		defer os.RemoveAll(tempDir)

		htpasswdTar = generateHtpasswd(t, tempDir, r.username, r.password)
		defer htpasswdTar.Close()

		otherEnvs := []string{
			"REGISTRY_AUTH=htpasswd",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm",
			"REGISTRY_AUTH_HTPASSWD_PATH=/registry_test_htpasswd",
		}
		registryEnv = append(registryEnv, otherEnvs...)
	}

	// Create container
	ctx := context.Background()
	ctr, err := DockerCli(t).ContainerCreate(ctx, &container.Config{
		Image: registryImageName,
		Env:   registryEnv,
	}, &container.HostConfig{
		AutoRemove: true,
		PortBindings: nat.PortMap{
			"5000/tcp": []nat.PortBinding{{}},
		},
	}, nil, r.Name)
	AssertNil(t, err)

	if r.username != "" {
		// Copy htpasswdTar to container
		AssertNil(t, DockerCli(t).CopyToContainer(ctx, ctr.ID, "/", htpasswdTar, types.CopyToContainerOptions{}))
	}

	// Start container
	AssertNil(t, DockerCli(t).ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{}))

	// Get port
	inspect, err := DockerCli(t).ContainerInspect(ctx, ctr.ID)
	AssertNil(t, err)
	r.Port = inspect.NetworkSettings.Ports["5000/tcp"][0].HostPort

	var authHeaders map[string]string
	if r.username != "" {
		// Write Docker config and configure auth headers
		writeDockerConfig(t, r.DockerDirectory, r.Host, r.Port, r.encodedAuth())
		authHeaders = map[string]string{"Authorization": "Basic " + r.encodedAuth()}
	}

	// Wait for registry to be ready
	Eventually(t, func() bool {
		txt, err := HTTPGetE(fmt.Sprintf("http://%s:%s/v2/_catalog", r.Host, r.Port), authHeaders)
		return err == nil && txt != ""
	}, 100*time.Millisecond, 10*time.Second)
}

func (r *DockerRegistry) Stop(t *testing.T) {
	t.Log("stop registry")
	t.Helper()
	if r.Name != "" {
		DockerCli(t).ContainerKill(context.Background(), r.Name, "SIGKILL")
		DockerCli(t).ContainerRemove(context.TODO(), r.Name, types.ContainerRemoveOptions{Force: true})
	}
}

func (r *DockerRegistry) RepoName(name string) string {
	return r.Host + ":" + r.Port + "/" + name
}

func (r *DockerRegistry) EncodedLabeledAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"username":"%s","password":"%s"}`, r.username, r.password)))
}

func DockerHostname(t *testing.T) string {
	dockerCli := DockerCli(t)

	daemonHost := dockerCli.DaemonHost()
	u, err := url.Parse(daemonHost)
	if err != nil {
		t.Fatalf("unable to parse URI client.DaemonHost: %s", err)
	}

	// if DOCKER_HOST is set to tcp, assume it is the host
	// Note: requires "insecure-registries" CIDR entry on Daemon config
	if u.Scheme == "tcp" {
		return u.Hostname()
	}

	// if host.docker.internal resolves, assume it's the host (https://docs.docker.com/docker-for-windows/networking/#use-cases-and-workarounds)
	// Note: requires "insecure-registries" CIDR entry on Daemon config
	addrs, err := net.LookupHost("host.docker.internal")
	if err == nil && len(addrs) == 1 {
		return addrs[0]
	}

	// if daemon has insecure registry entry with /32, assume it's the host
	daemonInfo, err := dockerCli.Info(context.TODO())
	if err != nil {
		t.Fatalf("unable to fetch client.DockerInfo: %s", err)
	}
	for _, ipnet := range daemonInfo.RegistryConfig.InsecureRegistryCIDRs {
		ones, _ := ipnet.Mask.Size()
		if ones == 32 {
			fmt.Printf("")
			return ipnet.IP.String()
		}
	}

	// Fallback to localhost, only works for Linux using --network=host
	return "localhost"
}

func (r *DockerRegistry) encodedAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", r.username, r.password)))
}

func generateHtpasswd(t *testing.T, tempDir string, username string, password string) io.ReadCloser {
	// https://docs.docker.com/registry/deploying/#restricting-access
	// HTPASSWD format: https://github.com/foomo/htpasswd/blob/e3a90e78da9cff06a83a78861847aa9092cbebdd/hashing.go#L23
	passwordBytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	return CreateSingleFileTarReader("/registry_test_htpasswd", username+":"+string(passwordBytes))
}

func writeDockerConfig(t *testing.T, configDir, host, port, auth string) {
	AssertNil(t, ioutil.WriteFile(
		filepath.Join(configDir, "config.json"),
		[]byte(fmt.Sprintf(`{
			  "auths": {
			    "%s:%s": {
			      "auth": "%s"
			    }
			  }
			}
			`, host, port, auth)),
		0666,
	))
}
