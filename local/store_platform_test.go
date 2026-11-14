package local

import (
	"context"
	"errors"
	"testing"

	"github.com/moby/moby/client"

	"github.com/buildpacks/imgutil"
)

// recordingDockerClient records how ImageSave was called and then fails, so the
// test can assert on the call without having to produce a valid image tar.
type recordingDockerClient struct {
	DockerClient

	called       bool
	saveOptCount int
}

func (c *recordingDockerClient) ImageSave(_ context.Context, _ []string, opts ...client.ImageSaveOption) (client.ImageSaveResult, error) {
	c.called = true
	c.saveOptCount = len(opts)
	return nil, errors.New("recorded; stopping before the tar is read")
}

// The daemon rejects a partially specified platform with "both OS and
// Architecture must be provided". s.platform is built from `docker inspect` of
// the base image, and an image that was not built for a specific platform
// reports an empty Architecture, so the store must not forward it.
func TestDoDownloadLayersForPlatform(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform imgutil.Platform
		wantOpts int
	}{
		{"complete platform is forwarded", imgutil.Platform{OS: "linux", Architecture: "amd64"}, 1},
		{"platform missing architecture is not forwarded", imgutil.Platform{OS: "linux"}, 0},
		{"platform missing os is not forwarded", imgutil.Platform{Architecture: "amd64"}, 0},
		{"zero platform is not forwarded", imgutil.Platform{}, 0},
		{"only osversion and variant is not forwarded", imgutil.Platform{OSVersion: "10.0.17763.1234", Variant: "v8"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &recordingDockerClient{}
			store := NewStoreWithPlatform(fake, tc.platform)

			// The error is expected: the fake stops once it has recorded the call.
			_ = store.doDownloadLayersFor("some-image-id")

			if !fake.called {
				t.Fatal("expected ImageSave to be called")
			}
			if fake.saveOptCount != tc.wantOpts {
				t.Fatalf("ImageSave save options: got %d, want %d", fake.saveOptCount, tc.wantOpts)
			}
		})
	}
}
