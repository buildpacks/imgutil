package index

import (
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/docker"
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

	layoutPath := filepath.Join(idxOps.xdgPath, idxOps.repoName)
	switch idxOps.format {
	case types.DockerManifestList:
		idx = &imgutil.Index{
			ImageIndex: docker.DockerIndex,
			Options: imgutil.IndexOptions{
				KeyChain:         idxOps.keychain,
				XdgPath:          idxOps.xdgPath,
				Reponame:         idxOps.repoName,
				InsecureRegistry: idxOps.insecure,
			},
			Images: make(map[v1.Hash]v1.Image),
		}
		_, err = layout.Write(layoutPath, docker.DockerIndex)
	default:
		idx = &imgutil.Index{
			ImageIndex: empty.Index,
			Annotate: imgutil.Annotate{
				Instance: make(map[v1.Hash]v1.Descriptor),
			},
			RemovedManifests: make([]v1.Hash, 10),
			Options: imgutil.IndexOptions{
				KeyChain:         idxOps.keychain,
				XdgPath:          idxOps.xdgPath,
				Reponame:         idxOps.repoName,
				InsecureRegistry: idxOps.insecure,
			},
			Images: make(map[v1.Hash]v1.Image),
		}
		_, err = layout.Write(layoutPath, empty.Index)
	}

	return idx, err
}
