package local

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// fixedLayer is a minimal v1.Layer with a known diffID and content.
type fixedLayer struct {
	diffID  v1.Hash
	content []byte
}

func (l fixedLayer) Digest() (v1.Hash, error) { return l.diffID, nil }
func (l fixedLayer) DiffID() (v1.Hash, error) { return l.diffID, nil }
func (l fixedLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.content)), nil
}
func (l fixedLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.content)), nil
}
func (l fixedLayer) Size() (int64, error)                { return int64(len(l.content)), nil }
func (l fixedLayer) MediaType() (types.MediaType, error) { return types.DockerLayer, nil }

// The daemon rejects tar entries with an absolute name:
//
//	embedded daemon response: invalid entry name "/sha256:<hex>.tar"
//
// manifest.json Layers paths are relative to the root of the tar, so the entry
// name written for a layer must be relative too -- as the config entry and the
// blank layers already are.
func TestAddLayerToTarEntryNameIsRelative(t *testing.T) {
	diffID := v1.Hash{
		Algorithm: "sha256",
		Hex:       "067833dadeca9180bb71a211248cbc0f6ada05499bfdef07dfb04e2eb96e7c82",
	}
	layer := fixedLayer{diffID: diffID, content: []byte("layer-bytes")}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	store := NewStore(nil)
	layerName, err := store.addLayerToTar(tw, layer, 0)
	if err != nil {
		t.Fatalf("addLayerToTar: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if strings.HasPrefix(layerName, "/") {
		t.Errorf("layer name recorded in manifest.json is absolute: %q", layerName)
	}
	if want := diffID.String() + ".tar"; layerName != want {
		t.Errorf("layer name: got %q, want %q", layerName, want)
	}

	// The name written into the archive must match the one reported back, or
	// manifest.json will point at an entry that does not exist.
	tr := tar.NewReader(&buf)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		names = append(names, hdr.Name)
		if strings.HasPrefix(hdr.Name, "/") {
			t.Errorf("tar entry name is absolute: %q", hdr.Name)
		}
	}
	if len(names) != 1 || names[0] != layerName {
		t.Errorf("tar entries = %v, want exactly [%q]", names, layerName)
	}
}
