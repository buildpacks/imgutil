package index

import (
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

// NewIndex will return a New Empty ImageIndex that can be modified and saved to a registry
func NewIndex(repoName string, ops ...Option) (idx *ImageIndex, err error) {
	var idxOps = &imgutil.IndexOptions{}
	for _, op := range ops {
		if err = op(idxOps); err != nil {
			return idx, err
		}
	}

	if err = ValidateRepoName(repoName, idxOps); err != nil {
		return idx, err
	}

	layoutPath := filepath.Join(idxOps.XdgPath, imgutil.MakeFileSafeName(repoName))

	var cnbIndex *imgutil.CNBIndex
	switch idxOps.Format {
	case types.DockerManifestList:
		cnbIndex, err = imgutil.NewCNBIndex(repoName, imgutil.NewEmptyDockerIndex(), *idxOps)
		if err != nil {
			return idx, err
		}
		// TODO I don't think we should write into disk during creation
		_, err = layout.Write(layoutPath, imgutil.NewEmptyDockerIndex())
	default:
		cnbIndex, err = imgutil.NewCNBIndex(repoName, imgutil.NewEmptyDockerIndex(), *idxOps)
		if err != nil {
			return idx, err
		}
		// TODO I don't think we should write into disk during creation
		_, err = layout.Write(layoutPath, empty.Index)
	}

	idx = &ImageIndex{
		CNBIndex: cnbIndex,
	}
	return idx, err
}
