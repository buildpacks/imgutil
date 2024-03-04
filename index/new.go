package index

import (
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

// NewIndex will return a New Empty ImageIndex that can be modified and saved to a registry
func NewIndex(repoName string, ops ...Option) (idx imgutil.ImageIndex, err error) {
	var idxOps = &Options{}
	ops = append(ops, WithRepoName(repoName))
	for _, op := range ops {
		err = op(idxOps)
		if err != nil {
			return
		}
	}

	idxOptions := imgutil.IndexOptions{
		KeyChain:         idxOps.keychain,
		XdgPath:          idxOps.xdgPath,
		Reponame:         idxOps.repoName,
		InsecureRegistry: idxOps.insecure,
	}

	layoutPath := filepath.Join(idxOps.xdgPath, idxOps.repoName)
	switch idxOps.format {
	case types.DockerManifestList:
		idx = imgutil.NewManifestHandler(imgutil.EmptyDocker(), idxOptions)
		_, err = layout.Write(layoutPath, imgutil.EmptyDocker())
	default:
		idx = imgutil.NewManifestHandler(empty.Index, idxOptions)
		_, err = layout.Write(layoutPath, empty.Index)
	}

	return idx, err
}
