package layout

import (
	"fmt"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

// NewIndex will return an OCI ImageIndex saved on disk using OCI media Types. It can be modified and saved to a registry
func NewIndex(repoName, path string, ops ...imgutil.IndexOption) (idx *ImageIndex, err error) {
	var mfest *v1.IndexManifest
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
		idxOps.BaseIndex, err = newImageIndexFromPath(idxOps.BaseImageIndexRepoName)
		if err != nil {
			return idx, err
		}

		if idxOps.BaseIndex != nil {
			// TODO Do we need to do this?
			mfest, err = idxOps.BaseIndex.IndexManifest()
			if err != nil {
				return idx, err
			}

			if mfest == nil {
				return idx, errors.New("encountered unexpected error while parsing image: manifest or index manifest is nil")
			}
		}
	}

	localPath := filepath.Join(path, imgutil.MakeFileSafeName(repoName))
	if idxOps.BaseIndex == nil {
		if imageExists(localPath) {
			return idx, errors.Errorf("an image index already exists at %s use FromBaseImageIndex or "+
				"FromBaseImageIndexInstance options to create a new instance", localPath)
		}
	}

	if idxOps.BaseIndex == nil {
		switch idxOps.MediaType {
		case types.DockerManifestList:
			idxOps.BaseIndex = imgutil.NewEmptyDockerIndex()
		default:
			idxOps.BaseIndex = empty.Index
		}
	}

	var cnbIndex *imgutil.CNBIndex
	idxOps.XdgPath = path
	cnbIndex, err = imgutil.NewCNBIndex(repoName, idxOps.BaseIndex, *idxOps)
	if err != nil {
		return idx, err
	}
	return &ImageIndex{
		CNBIndex: cnbIndex,
	}, nil
}

// newImageIndexFromPath creates a layout image index from the given path.
func newImageIndexFromPath(path string) (v1.ImageIndex, error) {
	if !imageExists(path) {
		return nil, nil
	}

	layoutPath, err := FromPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load layout from path: %w", err)
	}
	return layoutPath.ImageIndex()
}

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
	return nil
}
