package imgutil

import v1 "github.com/google/go-containerregistry/pkg/v1"

func NewIndexHandler(ii v1.ImageIndex, ops IndexOptions) *IndexHandler {
	return &IndexHandler{
		ImageIndex: ii,
		Options:    ops,
		Annotate: Annotate{
			Instance: make(map[v1.Hash]v1.Descriptor),
		},
		RemovedManifests: make([]v1.Hash, 0),
		Images:           make(map[v1.Hash]v1.Image),
	}
}

func NewManifestHandler(ii v1.ImageIndex, ops IndexOptions) *ManifestHandler {
	return &ManifestHandler{
		ImageIndex: ii,
		Options:    ops,
		Annotate: Annotate{
			Instance: make(map[v1.Hash]v1.Descriptor),
		},
		RemovedManifests: make([]v1.Hash, 0),
		Images:           make(map[v1.Hash]v1.Descriptor),
	}
}
