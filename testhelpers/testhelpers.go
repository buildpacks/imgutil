package testhelpers

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockercli "github.com/docker/docker/client"
	"github.com/google/go-cmp/cmp"
)

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(rand.Intn(26))
	}
	return string(b)
}

// Assert deep equality (and provide useful difference as a test failure)
func AssertEq(t *testing.T, actual, expected interface{}) {
	t.Helper()
	if diff := cmp.Diff(actual, expected); diff != "" {
		t.Fatal(diff)
	}
}

func AssertContains(t *testing.T, slice []string, elements ...string) {
	t.Helper()

outer:
	for _, el := range elements {
		for _, actual := range slice {
			if diff := cmp.Diff(actual, el); diff == "" {
				continue outer
			}
		}

		t.Fatalf("Expected %+v to contain: %s", slice, el)
	}
}

func AssertDoesNotContain(t *testing.T, slice []string, elements ...string) {
	t.Helper()

	for _, el := range elements {
		for _, actual := range slice {
			if diff := cmp.Diff(actual, el); diff == "" {
				t.Fatalf("Expected %+v to NOT contain: %s", slice, el)
			}
		}
	}
}

func AssertMatch(t *testing.T, actual string, expected *regexp.Regexp) {
	t.Helper()
	if !expected.Match([]byte(actual)) {
		t.Fatal(cmp.Diff(actual, expected))
	}
}

func AssertError(t *testing.T, actual error, expected string) {
	t.Helper()
	if actual == nil {
		t.Fatalf("Expected an error but got nil")
	}
	if !strings.Contains(actual.Error(), expected) {
		t.Fatalf(`Expected error to contain "%s", got "%s"`, expected, actual.Error())
	}
}

func AssertNil(t *testing.T, actual interface{}) {
	t.Helper()
	if actual != nil {
		t.Fatalf("Expected nil: %s", actual)
	}
}

var dockerCliVal *dockercli.Client
var dockerCliOnce sync.Once

func DockerCli(t *testing.T) *dockercli.Client {
	dockerCliOnce.Do(func() {
		var dockerCliErr error
		dockerCliVal, dockerCliErr = dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion("1.38"))
		AssertNil(t, dockerCliErr)
	})
	return dockerCliVal
}

func Eventually(t *testing.T, test func() bool, every time.Duration, timeout time.Duration) {
	t.Helper()

	ticker := time.NewTicker(every)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ticker.C:
			if test() {
				return
			}
		case <-timer.C:
			t.Fatalf("timeout on eventually: %v", timeout)
		}
	}
}

func CreateImageOnLocal(t *testing.T, dockerCli *dockercli.Client, repoName, dockerFile string, labels map[string]string) {
	ctx := context.Background()

	buildContext, err := CreateSingleFileTar("Dockerfile", dockerFile)
	AssertNil(t, err)

	res, err := dockerCli.ImageBuild(ctx, buildContext, dockertypes.ImageBuildOptions{
		Tags:           []string{repoName},
		SuppressOutput: true,
		Remove:         true,
		ForceRemove:    true,
		Labels:         labels,
	})
	AssertNil(t, err)

	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()
}

func CreateImageOnRemote(t *testing.T, dockerCli *dockercli.Client, repoName, dockerFile string, labels map[string]string) string {
	t.Helper()
	defer DockerRmi(dockerCli, repoName)

	CreateImageOnLocal(t, dockerCli, repoName, dockerFile, labels)

	var topLayer string
	inspect, _, err := dockerCli.ImageInspectWithRaw(context.TODO(), repoName)
	AssertNil(t, err)
	if len(inspect.RootFS.Layers) > 0 {
		topLayer = inspect.RootFS.Layers[len(inspect.RootFS.Layers)-1]
	} else {
		topLayer = "N/A"
	}

	AssertNil(t, pushImage(dockerCli, repoName))

	return topLayer
}

func PullImage(dockerCli *dockercli.Client, ref string) error {
	rc, err := dockerCli.ImagePull(context.Background(), ref, dockertypes.ImagePullOptions{})
	if err != nil {
		// Retry
		rc, err = dockerCli.ImagePull(context.Background(), ref, dockertypes.ImagePullOptions{})
		if err != nil {
			return err
		}
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		return err
	}
	return rc.Close()
}

func DockerRmi(dockerCli *dockercli.Client, repoNames ...string) error {
	var err error
	ctx := context.Background()
	for _, name := range repoNames {
		_, e := dockerCli.ImageRemove(
			ctx,
			name,
			dockertypes.ImageRemoveOptions{Force: true, PruneChildren: true},
		)
		if e != nil && err == nil {
			err = e
		}
	}
	return err
}

func CopySingleFileFromContainer(dockerCli *dockercli.Client, ctrID, path string) (string, error) {
	r, _, err := dockerCli.CopyFromContainer(context.Background(), ctrID, path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	tr := tar.NewReader(r)
	hdr, err := tr.Next()
	if hdr.Name != path && hdr.Name != filepath.Base(path) {
		return "", fmt.Errorf("filenames did not match: %s and %s (%s)", hdr.Name, path, filepath.Base(path))
	}
	b, err := ioutil.ReadAll(tr)
	return string(b), err
}

func CopySingleFileFromImage(dockerCli *dockercli.Client, repoName, path string) (string, error) {
	ctr, err := dockerCli.ContainerCreate(context.Background(),
		&container.Config{
			Image: repoName,
		}, &container.HostConfig{
			AutoRemove: true,
		}, nil, "",
	)
	if err != nil {
		return "", err
	}
	defer dockerCli.ContainerRemove(context.Background(), ctr.ID, dockertypes.ContainerRemoveOptions{})
	return CopySingleFileFromContainer(dockerCli, ctr.ID, path)
}

func pushImage(dockerCli *dockercli.Client, ref string) error {
	rc, err := dockerCli.ImagePush(context.Background(), ref, dockertypes.ImagePushOptions{RegistryAuth: "{}"})
	if err != nil {
		return err
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		return err
	}
	return rc.Close()
}

func HttpGetE(url string) (string, error) {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP Status was bad: %s => %d", url, resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ImageID(t *testing.T, repoName string) string {
	t.Helper()
	inspect, _, err := DockerCli(t).ImageInspectWithRaw(context.Background(), repoName)
	AssertNil(t, err)
	return inspect.ID
}

func CreateSingleFileTar(path, txt string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: path, Size: int64(len(txt)), Mode: 0644}); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(txt)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}
