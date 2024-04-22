package remote

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/buildpacks/imgutil"
)

// NewIndex returns a new ImageIndex from the registry that can be modified and saved to the local file system.
func NewIndex(repoName string, ops ...imgutil.IndexOption) (idx *ImageIndex, err error) {
	var idxOps = &imgutil.IndexOptions{}
	for _, op := range ops {
		if err = op(idxOps); err != nil {
			return idx, err
		}
	}

	if err = validateRepoName(repoName, idxOps); err != nil {
		return idx, err
	}

	if idxOps.BaseIndex == nil && idxOps.BaseImageIndexRepoName != "" {
		ref, err := name.ParseReference(idxOps.BaseImageIndexRepoName, name.WeakValidation, name.Insecure)
		if err != nil {
			return idx, err
		}

		desc, err := remote.Get(
			ref,
			remote.WithAuthFromKeychain(idxOps.KeyChain),
			remote.WithTransport(imgutil.GetTransport(idxOps.Insecure)),
		)
		if err != nil {
			return idx, err
		}

		idxOps.BaseIndex, err = desc.ImageIndex()
		if err != nil {
			return idx, err
		}
	}

	cnbIndex, err := imgutil.NewCNBIndex(repoName, idxOps.BaseIndex, *idxOps)
	if err != nil {
		return idx, err
	}

	return &ImageIndex{
		CNBIndex: cnbIndex,
	}, nil
}

// ValidateRepoName
// TODO move this code to something more generic
func validateRepoName(repoName string, o *imgutil.IndexOptions) error {
	if o.Insecure {
		_, err := name.ParseReference(repoName, name.Insecure, name.WeakValidation)
		if err != nil {
			return err
		}
	} else {
		_, err := name.ParseReference(repoName, name.WeakValidation)
		if err != nil {
			return err
		}
	}
	o.BaseImageIndexRepoName = repoName
	return nil
}
