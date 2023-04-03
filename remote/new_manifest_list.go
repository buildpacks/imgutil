package remote

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func NewIndex(repoName string, ops ...ImageOption) (*ImageIndex, error) {

	mediaType := defaultMediaType()

	index, err := emptyIndex(mediaType)
	if err != nil {
		return nil, err
	}

	ridx := &ImageIndex{
		repoName: repoName,
		index:    index,
	}

	return ridx, nil

}

func emptyIndex(mediaType types.MediaType) (v1.ImageIndex, error) {
	return mutate.IndexMediaType(empty.Index, mediaType), nil
}

func defaultMediaType() types.MediaType {
	return types.OCIImageIndex
}
