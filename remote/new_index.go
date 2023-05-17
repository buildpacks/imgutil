package remote

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

func NewIndex(repoName string, keychain authn.Keychain, ops ...ImageIndexOption) (*ImageIndex, error) {
	ref, err := name.ParseReference(repoName, name.WeakValidation)
	if err != nil {
		return nil, err
	}

	indexOpts := &indexOptions{}
	for _, op := range ops {
		if err := op(indexOpts); err != nil {
			return nil, err
		}
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(keychain))
	if err == nil {
		index, err := desc.ImageIndex()
		if err != nil {
			return nil, err
		}

		idx := &ImageIndex{
			keychain: keychain,
			repoName: repoName,
			index:    index,
		}

		return idx, nil
	}

	if len(indexOpts.manifest.Manifests) != 0 {
		index, err := emptyIndex(indexOpts.manifest.MediaType)
		if err != nil {
			return nil, err
		}

		for _, manifest_i := range indexOpts.manifest.Manifests {
			img, _ := emptyImage(imgutil.Platform{
				Architecture: manifest_i.Platform.Architecture,
				OS:           manifest_i.Platform.OS,
				OSVersion:    manifest_i.Platform.OSVersion,
			})
			index = mutate.AppendManifests(index, mutate.IndexAddendum{Add: img, Descriptor: manifest_i})
		}

		idx := &ImageIndex{
			keychain: keychain,
			repoName: repoName,
			index:    index,
		}

		return idx, nil

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

func NewIndexTest(repoName string, keychain authn.Keychain, ops ...ImageIndexOption) (*ImageIndexTest, error) {
	ridx, err := NewIndex(repoName, keychain, ops...)
	if err != nil {
		return nil, err
	}

	ridxt := &ImageIndexTest{
		ImageIndex: *ridx,
	}

	return ridxt, nil
}
