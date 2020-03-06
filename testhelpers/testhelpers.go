package testhelpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
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

func AssertNotEq(t *testing.T, v1, v2 interface{}) {
	t.Helper()

	if diff := cmp.Diff(v1, v2); diff == "" {
		t.Fatalf("expected values not to be equal, both equal to %v", v1)
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
		t.Fatalf(
			`Expected error to contain "%s", got "%s"\n\n Diff:\n%s`,
			expected,
			actual.Error(),
			cmp.Diff(expected, actual.Error()),
		)
	}
}

func AssertNil(t *testing.T, actual interface{}) {
	t.Helper()
	if actual != nil {
		t.Fatalf("Expected nil: %s", actual)
	}
}

var dockerCliVal dockercli.CommonAPIClient
var dockerCliOnce sync.Once

func DockerCli(t *testing.T) dockercli.CommonAPIClient {
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

func CreateImageOnLocal(t *testing.T, dockerCli dockercli.CommonAPIClient, repoName, dockerFile string, labels map[string]string) {
	t.Helper()
	ctx := context.Background()

	buildContext, err := CreateSingleFileTar("Dockerfile", dockerFile, "linux")
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

func CreateImageOnRemote(t *testing.T, dockerCli dockercli.CommonAPIClient, repoName, dockerFile, registryAuth string, labels map[string]string) {
	t.Helper()
	defer DockerRmi(dockerCli, repoName)

	CreateImageOnLocal(t, dockerCli, repoName, dockerFile, labels)

	PushImage(dockerCli, repoName, registryAuth)
}

func PullImage(dockerCli dockercli.CommonAPIClient, ref string, registryAuth ...string) error {
	pullOptions := dockertypes.ImagePullOptions{}
	if len(registryAuth) == 1 {
		pullOptions = dockertypes.ImagePullOptions{RegistryAuth: registryAuth[0]}
	}

	rc, err := dockerCli.ImagePull(context.Background(), ref, pullOptions)
	if err != nil {
		// Retry
		rc, err = dockerCli.ImagePull(context.Background(), ref, pullOptions)
		if err != nil {
			return err
		}
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		return err
	}
	return rc.Close()
}

func DockerRmi(dockerCli dockercli.CommonAPIClient, repoNames ...string) error {
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

func CopySingleFileFromContainer(dockerCli dockercli.CommonAPIClient, ctrID, path string) (string, error) {
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

func CopySingleFileFromImage(dockerCli dockercli.CommonAPIClient, repoName, path string) (string, error) {
	ctr, err := dockerCli.ContainerCreate(context.Background(),
		&dockercontainer.Config{
			Image: repoName,
		}, &dockercontainer.HostConfig{
			AutoRemove: true,
		}, nil, "",
	)
	if err != nil {
		return "", err
	}
	defer dockerCli.ContainerRemove(context.Background(), ctr.ID, dockertypes.ContainerRemoveOptions{})
	return CopySingleFileFromContainer(dockerCli, ctr.ID, path)
}

func PushImage(dockerCli dockercli.CommonAPIClient, ref string, registryAuth ...string) error {
	pushOptions := dockertypes.ImagePushOptions{RegistryAuth: "{}"}
	if len(registryAuth) == 1 {
		pushOptions = dockertypes.ImagePushOptions{RegistryAuth: registryAuth[0]}
	}

	rc, err := dockerCli.ImagePush(context.Background(), ref, pushOptions)
	if err != nil {
		return err
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		return err
	}
	return rc.Close()
}

func HttpGetAuthE(url, username, password string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(username, password)

	resp, err := http.DefaultClient.Do(req)
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

func WriteWindowsBaseLayerTar(outputPath string) error {
	tarFile, err := os.OpenFile(outputPath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(tarFile)

	//Valid BCD file required, containing Windows Boot Manager and Windows Boot Loader sections
	//Note: Gzip/Base64 encoded only to inline the binary BCD file here
	//CMD: `bcdedit /createstore c:\output-bcd & bcdedit /create {6a6c1f1b-59d4-11ea-9438-9402e6abd998} /d buildpacks.io /application osloader /store c:\output-bcd & bcdedit /create {bootmgr} /store c:\output-bcd & bcdedit /set {bootmgr} default {6a6c1f1b-59d4-11ea-9438-9402e6abd998} /store c:\output-bcd & bcdedit /enum all /store c:\output-bcd`
	//BASH: `gzip --stdout --best output-bcd | base64`
	bcdGzipBase64 := "H4sIABeDWF4CA+1YTWgTQRR+m2zSFItGkaJQcKXgyZX8bNKklxatUvyJoh4qKjS7O7GxzQ9Jai210JtFQXrUW4/e2ostCF4EoSCCF6HHQi9FWsxJepH43s5us02XYlFBcL7l7U7evHnzzZtvoJ0Ke5A7BABkoU/f3qxvfZEkbPuBg9oKNcK8fQ/68LkHBvTiuwTjUIOy9VZBR68J++NZ/sXU/J2vR99d/thxdzOz0PqbYp63+CqFWuH7wg+LGwj8MfRuvnovqiAgICAgICAgICAgIPB/YETPF8H+/96Bcw9A7flGo1EcPQvrRVginw99Q6cAfHbsEDYwpEFt+s7Y306PuTrQMmziVq1UYTdLpRr5HmNsdRSg38+N8o5Z9w7yzCDlt8ceB+rT7HlPQB8cAYn/CND9hOLjUZaf3xIEjlGelhiJX2wE7gOfy7lQeG2tU4Gt7vZlZ50KNNd5I7B7ncSVvlc91tmGdl1/yIxaFbYxph4EmLXzq/2nd/KHpGZ+RfbO71XHM2hTyWzSiOaiuppIm5oajbKsmtbiKXxFYiyZ1c10OjUNUMccZZxkGDkMsKpBfBS/s68KPHlbX3Kv10HDBvnXkKezr06/Yu0CB90dUe5KvlzLl7icNjB2LOeDbYn30VqpJmvofzTaZo2d8/H6k11hk5lsgQH1n4cLMACRlofDi/eItJc3ueoS15SbdwhN3od33eItQQqDorFIhPOVacyMH5SwbPO9PVlmgy79Un3I2n9Rvych+Ff0+6Grc9ldF6c/7PfWV9hDX1Sji2OswIq1qrOPiz5eqxU/7/Oab8XvvQ+f5b37cBityzUf1RqhOfqgvkW5qQ+bj6UPHcYhj1U2oQxZMGAUqnAOPSWMI33Pyc3z5j7P7vM2GDzgeUubLJtKxgw1YZimqrGeiJo1jKiai8f0uKaZWk86Md3UPdWezuiqzMd66XZV9q7XRuDgunXr1AfhXToFuy4rgaZOup82dvFwdLIW/D2djAQ4t3qA961avMIWL8leA30v5SuFiWyFXSuZ+VyemV68KIdXfYYlbz1lXLxicUtPcec8zwa5z9EXxYbb9uqLeExBEnWVRGVFIYemgwoJSKPeNGxF8WHYr6JHgzik7FYEYuinkTpGpvFJwfQO/5ch8beGgICAgICAwL+BnwAgqcMAIAAA"
	bcdGzip, _ := base64.StdEncoding.DecodeString(bcdGzipBase64)
	bcdReader, _ := gzip.NewReader(bytes.NewBuffer(bcdGzip))
	bcdBytes, _ := ioutil.ReadAll(bcdReader)

	tw.WriteHeader(&tar.Header{Name: "Files", Typeflag: tar.TypeDir})
	tw.WriteHeader(&tar.Header{Name: "Files/Windows", Typeflag: tar.TypeDir})
	tw.WriteHeader(&tar.Header{Name: "Files/Windows/System32", Typeflag: tar.TypeDir})
	tw.WriteHeader(&tar.Header{Name: "Files/Windows/System32/config", Typeflag: tar.TypeDir})

	tw.WriteHeader(&tar.Header{Name: "UtilityVM", Typeflag: tar.TypeDir})
	tw.WriteHeader(&tar.Header{Name: "UtilityVM/Files", Typeflag: tar.TypeDir})
	tw.WriteHeader(&tar.Header{Name: "UtilityVM/Files/EFI", Typeflag: tar.TypeDir})
	tw.WriteHeader(&tar.Header{Name: "UtilityVM/Files/EFI/Microsoft", Typeflag: tar.TypeDir})
	tw.WriteHeader(&tar.Header{Name: "UtilityVM/Files/EFI/Microsoft/Boot", Typeflag: tar.TypeDir})

	tw.WriteHeader(&tar.Header{Name: "Files/Windows/System32/config/DEFAULT", Size: 0, Mode: 0644})
	tw.WriteHeader(&tar.Header{Name: "Files/Windows/System32/config/SAM", Size: 0, Mode: 0644})
	tw.WriteHeader(&tar.Header{Name: "Files/Windows/System32/config/SECURITY", Size: 0, Mode: 0644})
	tw.WriteHeader(&tar.Header{Name: "Files/Windows/System32/config/SOFTWARE", Size: 0, Mode: 0644})
	tw.WriteHeader(&tar.Header{Name: "Files/Windows/System32/config/SYSTEM", Size: 0, Mode: 0644})

	tw.WriteHeader(&tar.Header{Name: "UtilityVM/Files/EFI/Microsoft/Boot/BCD", Size: int64(len(bcdBytes)), Mode: 0644})
	tw.Write(bcdBytes)

	if err := tw.Close(); err != nil {
		return err
	}

	return nil
}

func CreateSingleFileTar(containerPath, txt, osType string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	writeFunc := writeTarSingleFileLinux
	if osType == "windows" {
		writeFunc = writeTarSingleFileWindows
	}

	err := writeFunc(tw, containerPath, txt)
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

func writeTarSingleFileLinux(tw *tar.Writer, layerPath, txt string) error {
	if err := tw.WriteHeader(&tar.Header{Name: layerPath, Size: int64(len(txt)), Mode: 0644}); err != nil {
		return err
	}

	if _, err := tw.Write([]byte(txt)); err != nil {
		return err
	}

	return nil
}

func writeTarSingleFileWindows(tw *tar.Writer, containerPath, txt string) error {
	// root Windows layer directories
	if err := tw.WriteHeader(&tar.Header{Name: "Files", Typeflag: tar.TypeDir}); err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: "Hives", Typeflag: tar.TypeDir}); err != nil {
		return err
	}

	// prepend file entries with "Files"
	layerPath := path.Join("Files", containerPath)
	if err := tw.WriteHeader(&tar.Header{Name: layerPath, Size: int64(len(txt)), Mode: 0644}); err != nil {
		return err
	}

	if _, err := tw.Write([]byte(txt)); err != nil {
		return err
	}

	return nil
}

func FetchManifestLayers(t *testing.T, repoName, imageOS string, keychain authn.Keychain) []string {
	t.Helper()

	r, err := name.ParseReference(repoName, name.WeakValidation)
	AssertNil(t, err)

	gImg, err := remote.Image(
		r,
		remote.WithTransport(http.DefaultTransport),
		remote.WithPlatform(v1.Platform{OS: imageOS}),
		remote.WithAuthFromKeychain(keychain),
	)
	AssertNil(t, err)

	gLayers, err := gImg.Layers()
	AssertNil(t, err)

	var manifestLayers []string
	for _, gLayer := range gLayers {
		diffID, err := gLayer.DiffID()
		AssertNil(t, err)

		manifestLayers = append(manifestLayers, diffID.String())
	}

	return manifestLayers
}

func FetchManifestImageConfigFile(t *testing.T, repoName, imageOS string, keychain authn.Keychain) *v1.ConfigFile {
	t.Helper()

	r, err := name.ParseReference(repoName, name.WeakValidation)
	AssertNil(t, err)

	gImg, err := remote.Image(
		r,
		remote.WithTransport(http.DefaultTransport),
		remote.WithPlatform(v1.Platform{OS: imageOS}),
		remote.WithAuthFromKeychain(keychain),
	)
	AssertNil(t, err)

	configFile, err := gImg.ConfigFile()
	AssertNil(t, err)

	return configFile
}
