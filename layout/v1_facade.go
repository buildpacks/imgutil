package layout

import (
	"bytes"
	"fmt"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type v1LayerFacade struct {
	v1.Layer
	diffID v1.Hash
	digest v1.Hash
	size   int64
}

func (l *v1LayerFacade) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (l *v1LayerFacade) DiffID() (v1.Hash, error) {
	return l.diffID, nil
}

func (l *v1LayerFacade) Digest() (v1.Hash, error) {
	return l.digest, nil
}

func (l *v1LayerFacade) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (l *v1LayerFacade) Size() (int64, error) {
	return l.size, nil
}

func newLayerOrFacadeFrom(configFile v1.ConfigFile, manifestFile v1.Manifest, layerIndex int, originalLayer v1.Layer) (v1.Layer, error) {
	if hasData(originalLayer) {
		return originalLayer, nil
	}
	if layerIndex > len(configFile.RootFS.DiffIDs) {
		return nil, fmt.Errorf("failed to find layer for index %d in config file", layerIndex)
	}
	if layerIndex > (len(manifestFile.Layers)) {
		return nil, fmt.Errorf("failed to find layer for index %d in manifest file", layerIndex)
	}
	return &v1LayerFacade{
		Layer:  originalLayer,
		diffID: configFile.RootFS.DiffIDs[layerIndex],
		digest: manifestFile.Layers[layerIndex].Digest,
		size:   manifestFile.Layers[layerIndex].Size,
	}, nil
}

func hasData(layer v1.Layer) bool {
	if rc, err := layer.Compressed(); err == nil {
		defer rc.Close()
		return true
	}
	return false
}
