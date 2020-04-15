package testhelpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

func PullImage(dockerCli dockercli.CommonAPIClient, ref string) error {
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

func DockerRmi(dockerCli dockercli.CommonAPIClient, repoNames ...string) error {
	var err error
	ctx := context.Background()
	for _, name := range repoNames {
		_, e := dockerCli.ImageRemove(
			ctx,
			name,
			dockertypes.ImageRemoveOptions{PruneChildren: true},
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
	if err != nil {
		return "", err
	}
	if hdr.Name != path && hdr.Name != filepath.Base(path) {
		return "", fmt.Errorf("filenames did not match: %s and %s (%s)", hdr.Name, path, filepath.Base(path))
	}
	b, err := ioutil.ReadAll(tr)
	return string(b), err
}

func CreateContainer(dockerCli dockercli.CommonAPIClient, repoName string) (string, error) {
	ctr, err := dockerCli.ContainerCreate(context.Background(),
		&dockercontainer.Config{
			Image: repoName,
			Cmd:   []string{"noop"},
		}, &dockercontainer.HostConfig{
			AutoRemove: true,
			Isolation:  fastestIsolation(dockerCli),
		}, nil, "",
	)
	if err != nil {
		return "", err
	}
	return ctr.ID, nil
}

func fastestIsolation(dockerCli dockercli.CommonAPIClient) dockercontainer.Isolation {
	daemonInfo, err := dockerCli.Info(context.Background())
	if err != nil {
		panic(err)
	}

	if daemonInfo.OSType == "windows" {
		return dockercontainer.IsolationProcess
	}
	return dockercontainer.IsolationDefault
}

func CopySingleFileFromImage(dockerCli dockercli.CommonAPIClient, repoName, path string) (string, error) {
	ctrID, err := CreateContainer(dockerCli, repoName)
	if err != nil {
		return "", err
	}
	defer dockerCli.ContainerRemove(context.Background(), ctrID, dockertypes.ContainerRemoveOptions{})
	return CopySingleFileFromContainer(dockerCli, ctrID, path)
}

func PushImage(dockerCli dockercli.CommonAPIClient, ref string) error {
	rc, err := dockerCli.ImagePush(context.Background(), ref, dockertypes.ImagePushOptions{RegistryAuth: "{}"})
	if err != nil {
		return err
	}
	if _, err := io.Copy(ioutil.Discard, rc); err != nil {
		return err
	}
	return rc.Close()
}

func HTTPGetE(url string) (string, error) {
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

func CreateSingleFileBaseLayerTar(layerPath, txt, osType string) (string, error) {
	tarFile, err := ioutil.TempFile("", "create-base-layer-tar-path")
	if err != nil {
		return "", err
	}
	defer tarFile.Close()

	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	// regular Linux layer
	writeFunc := writeTarSingleFileLinux
	if osType == "windows" {
		// special Windows base layer
		writeFunc = writeTarWindowsBaseLayer
	}

	err = writeFunc(tw, layerPath, txt)
	if err != nil {
		return "", err
	}

	return tarFile.Name(), nil
}

func CreateSingleFileLayerTar(layerPath, txt, osType string) (string, error) {
	tarFile, err := ioutil.TempFile("", "create-single-file-layer-tar-path")
	if err != nil {
		return "", err
	}
	defer tarFile.Close()

	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	// regular Linux layer
	writeFunc := writeTarSingleFileLinux
	if osType == "windows" {
		// regular Windows layer
		writeFunc = writeTarSingleFileWindows
	}

	err = writeFunc(tw, layerPath, txt)
	if err != nil {
		return "", err
	}

	return tarFile.Name(), nil
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

// Windows image layers must follow this pattern¹:
// - base layer² (always required; tar file with relative paths without "/" prefix; all parent directories require own tar entries)
//   \-> Files/Windows/System32/config/DEFAULT   (file and must exist but can be empty)
//   \-> Files/Windows/System32/config/SAM       (file must exist but can be empty)
//   \-> Files/Windows/System32/config/SECURITY  (file must exist but can be empty)
//   \-> Files/Windows/System32/config/SOFTWARE  (file must exist but can be empty)
//   \-> Files/Windows/System32/config/SYSTEM    (file must exist but can be empty)
//   \-> UtilityVM/Files/EFI/Microsoft/Boot/BCD   (file must exist and a valid BCD format - via `bcdedit` tool as below)
// - normal or top layer (optional; tar file with relative paths without "/" prefix; all parent directories require own tar entries)
//   \-> Files/                   (required directory entry)
//   \-> Files/mystuff.exe        (optional container filesystem files - C:\mystuff.exe)
//   \-> Hives/                   (required directory entry)
//   \-> Hives/DefaultUser_Delta  (optional Windows reg hive delta; BCD format - HKEY_USERS\.DEFAULT additional content)
//   \-> Hives/Sam_Delta          (optional Windows reg hive delta; BCD format - HKEY_LOCAL_MACHINE\SAM additional content)
//   \-> Hives/Security_Delta     (optional Windows reg hive delta; BCD format - HKEY_LOCAL_MACHINE\SECURITY additional content)
//   \-> Hives/Software_Delta     (optional Windows reg hive delta; BCD format - HKEY_LOCAL_MACHINE\SOFTWARE additional content)
//   \-> Hives/System_Delta       (optional Windows reg hive delta; BCD format - HKEY_LOCAL_MACHINE\SYSTEM additional content)
// 1. This was all discovered experimentally and should be considered an undocumented API, subject to change when the Windows Daemon internals change
// 2. There are many other files in an "real" base layer but this is the minimum set which a Daemon can store and use to create an container
func writeTarWindowsBaseLayer(tw *tar.Writer, containerPath, txt string) error {
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

func FetchManifestLayers(t *testing.T, repoName string) []string {
	t.Helper()

	r, err := name.ParseReference(repoName, name.WeakValidation)
	AssertNil(t, err)

	gImg, err := remote.Image(
		r,
		remote.WithTransport(http.DefaultTransport),
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

func FetchManifestImageConfigFile(t *testing.T, repoName string) *v1.ConfigFile {
	t.Helper()

	r, err := name.ParseReference(repoName, name.WeakValidation)
	AssertNil(t, err)

	gImg, err := remote.Image(r, remote.WithTransport(http.DefaultTransport))
	AssertNil(t, err)

	configFile, err := gImg.ConfigFile()
	AssertNil(t, err)

	return configFile
}

func FileDiffID(path string) (string, error) {
	tarFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer tarFile.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, tarFile)
	if err != nil {
		return "", err
	}

	diffID := "sha256:" + hex.EncodeToString(hasher.Sum(make([]byte, 0, hasher.Size())))

	return diffID, nil
}

func RunnableBaseImage(os string) string {
	if os == "windows" {
		// windows/amd64 image from manifest cached on github actions Windows 2019 workers: https://github.com/actions/virtual-environments/blob/master/images/win/Windows2019-Readme.md#docker-images
		return "mcr.microsoft.com/windows/nanoserver@sha256:08c883692e527b2bb4d7f6579e7707a30a2aaa66556b265b917177565fd76117"
	}
	return "busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"
}
