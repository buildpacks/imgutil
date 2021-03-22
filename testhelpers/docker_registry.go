package testhelpers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"golang.org/x/crypto/bcrypt"

	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
)

type DockerRegistry struct {
	Host            string
	Port            string
	Name            string
	DockerDirectory string
	username        string
	password        string
	volumeName      string
}

var registryImageName = "micahyoung/registry:latest"

type RegistryOption func(registry *DockerRegistry)

//WithSharedStorageVolume allows two instances to share the same data volume.
//Use an authenticated registry to write to a read-only unauthenticated registry.
//Volumes that don't exist will be created, then removed on Stop().
func WithSharedStorageVolume(volumeName string) RegistryOption {
	return func(registry *DockerRegistry) {
		registry.volumeName = volumeName
	}
}

//WithAuth adds credentials to registry. Omitting will make the registry read-only
func WithAuth(dockerConfigDir string) RegistryOption {
	return func(r *DockerRegistry) {
		r.username = RandString(10)
		r.password = RandString(10)
		r.DockerDirectory = dockerConfigDir
	}
}

func NewDockerRegistry(ops ...RegistryOption) *DockerRegistry {
	registry := &DockerRegistry{
		Name: "test-registry-" + RandString(10),
	}

	for _, op := range ops {
		op(registry)
	}

	return registry
}

func (r *DockerRegistry) Start(t *testing.T) {
	t.Helper()

	r.Host = DockerHostname(t)

	t.Logf("run registry on %s", r.Host)

	PullIfMissing(t, DockerCli(t), registryImageName)

	registryEnv := []string{
		"REGISTRY_STORAGE_DELETE_ENABLED=true",
	}

	var htpasswdTar io.ReadCloser
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
	} else {
		// make read-only without auth
		readOnlyEnv := `REGISTRY_STORAGE_MAINTENANCE_READONLY={"enabled":true}`
		registryEnv = append(registryEnv, readOnlyEnv)
	}

	var volumeBinds []string
	var containerUser string
	if r.volumeName != "" {
		// try to create volumes that may exist
		_, err := DockerCli(t).VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{Name: r.volumeName})
		if err != nil {
			// fail if err is not from existing volume
			if !errdefs.IsConflict(err) {
				AssertNil(t, err)
			}
		}

		info, err := DockerCli(t).Info(context.Background())
		AssertNil(t, err)

		storageBindPath := "/registry-storage"
		if info.OSType == "windows" {
			containerUser = "ContainerAdministrator" //required for volume permissions
			storageBindPath = "c:/registry-storage"
		}

		volumeBinds = append(volumeBinds, fmt.Sprintf("%s:%s", r.volumeName, storageBindPath))

		storageEnv := fmt.Sprintf("REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=%s", storageBindPath)
		registryEnv = append(registryEnv, storageEnv)
	}

	// Create container
	ctx := context.Background()
	ctr, err := DockerCli(t).ContainerCreate(ctx, &container.Config{
		Image: registryImageName,
		Env:   registryEnv,
		User:  containerUser,
	}, &container.HostConfig{
		AutoRemove: true,
		PortBindings: nat.PortMap{
			"5000/tcp": []nat.PortBinding{{}},
		},
		Binds: volumeBinds,
	}, nil, r.Name)
	AssertNil(t, err)

	if r.username != "" {
		// Copy htpasswdTar to container
		AssertNil(t, DockerCli(t).CopyToContainer(ctx, ctr.ID, "/", htpasswdTar, types.CopyToContainerOptions{}))
	}

	// Start container
	AssertNil(t, DockerCli(t).ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{}))

	// Get port when ready
	for i := 0; i < 5; i++ {
		inspect, err := DockerCli(t).ContainerInspect(ctx, ctr.ID)
		AssertNil(t, err)

		hostPortMap := inspect.NetworkSettings.Ports["5000/tcp"]
		if len(hostPortMap) == 1 {
			r.Port = hostPortMap[0].HostPort
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

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
	t.Helper()
	t.Log("stop registry")

	if r.Name != "" {
		DockerCli(t).ContainerKill(context.Background(), r.Name, "SIGKILL")
		DockerCli(t).ContainerRemove(context.TODO(), r.Name, types.ContainerRemoveOptions{Force: true})
	}

	if r.volumeName != "" {
		// try to cleanup shared volume if this is the last user
		err := DockerCli(t).VolumeRemove(context.Background(), r.volumeName, false)
		if err != nil {
			// fail if err is not from volume in use
			if !errdefs.IsConflict(err) {
				AssertNil(t, err)
			}
		}
	}
}

func (r *DockerRegistry) RepoName(name string) string {
	return r.Host + ":" + r.Port + "/" + name
}

func (r *DockerRegistry) EncodedLabeledAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"username":"%s","password":"%s"}`, r.username, r.password)))
}

//DockerHostname discovers the appropriate registry hostname.
//For test to run where "localhost" is not the daemon host, a `insecure-registries` entry of `<host IP>/32` is required to allow test images to be written.
//For Docker Desktop, this can be set here: https://docs.docker.com/docker-for-mac/#docker-engine
//Otherwise, its set in the daemon.json: https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file
//If the entry is not found, the fallback is "localhost"
func DockerHostname(t *testing.T) string {
	dockerCli := DockerCli(t)

	// if daemon has insecure registry entry with /32, assume it is the host
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
