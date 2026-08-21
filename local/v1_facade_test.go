package local

import (
	"bytes"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil"
)

func TestImageFromPreservesDistinctLayers(t *testing.T) {
	first, err := v1.NewHash("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	second, err := v1.NewHash("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}

	image, err := imageFrom([]v1.Layer{
		facadeLayer(first, "first"),
		facadeLayer(second, "second"),
	}, &v1.ConfigFile{RootFS: v1.RootFS{Type: "layers"}}, imgutil.DockerTypes)
	if err != nil {
		t.Fatal(err)
	}

	layers, err := image.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Fatalf("got %d layers, want 2", len(layers))
	}

	for index, expected := range []v1.Hash{first, second} {
		actual, err := layers[index].DiffID()
		if err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Errorf("layer %d has diff ID %q, want %q", index, actual, expected)
		}
	}
}

func facadeLayer(diffID v1.Hash, contents string) *v1LayerFacade {
	return &v1LayerFacade{
		diffID: diffID,
		uncompressed: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBufferString(contents)), nil
		},
		uncompressedSize: func() (int64, error) {
			return int64(len(contents)), nil
		},
	}
}
