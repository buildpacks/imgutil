package local

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/buildpacks/imgutil"
)

func TestOCIPlatform(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform imgutil.Platform
		want     ocispec.Platform
		wantOK   bool
	}{
		{"complete", imgutil.Platform{OS: "linux", Architecture: "amd64"}, ocispec.Platform{OS: "linux", Architecture: "amd64"}, true},
		{"complete with variant and os version", imgutil.Platform{OS: "windows", Architecture: "arm", Variant: "v7", OSVersion: "10.0.17763.1040"},
			ocispec.Platform{OS: "windows", Architecture: "arm", Variant: "v7", OSVersion: "10.0.17763.1040"}, true},
		{"missing architecture", imgutil.Platform{OS: "linux"}, ocispec.Platform{}, false},
		{"missing os", imgutil.Platform{Architecture: "amd64"}, ocispec.Platform{}, false},
		{"only variant and os version", imgutil.Platform{OSVersion: "10.0.17763.1234", Variant: "v8"}, ocispec.Platform{}, false},
		{"zero", imgutil.Platform{}, ocispec.Platform{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ociPlatform(tc.platform)
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if got.OS != tc.want.OS || got.Architecture != tc.want.Architecture ||
				got.Variant != tc.want.Variant || got.OSVersion != tc.want.OSVersion {
				t.Errorf("platform: got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// versionDockerClient answers ServerVersion and fails the test if the daemon is
// asked about an image.
type versionDockerClient struct {
	DockerClient

	t          *testing.T
	apiVersion string
	os, arch   string
}

func (c *versionDockerClient) ServerVersion(context.Context, client.ServerVersionOptions) (client.ServerVersionResult, error) {
	return client.ServerVersionResult{APIVersion: c.apiVersion, Os: c.os, Arch: c.arch}, nil
}

func (c *versionDockerClient) ImageHistory(context.Context, string, ...client.ImageHistoryOption) (client.ImageHistoryResult, error) {
	c.t.Fatal("ImageHistory must not be called with a partial platform")
	return client.ImageHistoryResult{}, nil
}

func (c *versionDockerClient) ImageInspect(context.Context, string, ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	c.t.Fatal("ImageInspect must not be called with a partial platform")
	return client.ImageInspectResult{}, nil
}

// A new image takes its config platform from the requested one. A partial
// request must stay partial: pack builds buildpack packages without a target as
// {OS: linux}, and stamping the build machine's architecture on them would make
// an architecture-independent package claim a single architecture.
func TestNewImageKeepsPartialPlatformInConfig(t *testing.T) {
	dockerClient := &versionDockerClient{t: t, apiVersion: "1.56", os: "linux", arch: "amd64"}

	img, err := NewImage("some-image", dockerClient, imgutil.WithDefaultPlatform(imgutil.Platform{OS: "linux"}))
	if err != nil {
		t.Fatalf("NewImage: %v", err)
	}
	imgOS, err := img.OS()
	if err != nil {
		t.Fatalf("OS: %v", err)
	}
	arch, err := img.Architecture()
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}
	if imgOS != "linux" || arch != "" {
		t.Errorf("config platform: got %s/%q, want linux/\"\"", imgOS, arch)
	}
}

// The daemon rejects a partial platform on inspect and history the same way it
// does on save; treat it like a platform the image does not have, so the caller
// falls back to the generic inspect.
func TestGetPlatformAwareInspectAndHistorySkipsPartialPlatform(t *testing.T) {
	for _, platform := range []imgutil.Platform{
		{OS: "linux"},
		{Architecture: "amd64"},
		{},
	} {
		inspect, history, err := getPlatformAwareInspectAndHistory("some-image", platform, &versionDockerClient{t: t})
		if err != nil || inspect != nil || history != nil {
			t.Errorf("%+v: got (%v, %v, %v), want (nil, nil, nil)", platform, inspect, history, err)
		}
	}
}
