package remote

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

func NewIndex(repoName string, keychain authn.Keychain, ops ...ImageIndexOption) (*ImageIndex, error) {
	if _, err := name.ParseReference(repoName, name.WeakValidation); err != nil {
		return nil, err
	}

	indexOpts := &indexOptions{}
	for _, op := range ops {
		if err := op(indexOpts); err != nil {
			return nil, err
		}
	}

	mediaType := defaultMediaType()
	if indexOpts.mediaTypes.IndexManifestType() != "" {
		mediaType = indexOpts.mediaTypes
	}

	index, err := emptyIndex(mediaType.IndexManifestType())
	if err != nil {
		return nil, err
	}

	ridx := &ImageIndex{
		keychain: keychain,
		repoName: repoName,
		index:    index,
	}

	return ridx, nil

}

func emptyIndex(mediaType types.MediaType) (v1.ImageIndex, error) {
	return mutate.IndexMediaType(empty.Index, mediaType), nil
}

func defaultMediaType() imgutil.MediaTypes {
	return imgutil.DockerTypes
}
