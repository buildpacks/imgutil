package index

import (
	"encoding/json"
	"errors"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var DockerIndex = dockerIndex{}

type dockerIndex struct{}

func (i *dockerIndex) MediaType() (types.MediaType, error) {
	return types.DockerManifestList, nil
}

func (i *dockerIndex) Digest() (v1.Hash, error) {
	return partial.Digest(i)
}

func (i *dockerIndex) Size() (int64, error) {
	return partial.Size(i)
}

func (i *dockerIndex) IndexManifest() (*v1.IndexManifest, error) {
	return base(), nil
}

func (i *dockerIndex) RawManifest() ([]byte, error) {
	return json.Marshal(base())
}

func (i *dockerIndex) Image(v1.Hash) (v1.Image, error) {
	return nil, errors.New("empty index")
}

func (i *dockerIndex) ImageIndex(v1.Hash) (v1.ImageIndex, error) {
	return nil, errors.New("empty index")
}

func base() *v1.IndexManifest {
	return &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestList,
		Manifests:     []v1.Descriptor{},
	}
}