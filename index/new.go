package index

import (
	"errors"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/docker"
)

// NewIndex will return a New Empty ImageIndex that can be modified and saved to a registry
func NewIndex(repoName string, ops ...Option) (index imgutil.Index, err error) {
	var idxOps = &Options{}
	ops = append(ops, WithRepoName(repoName))
	for _, op := range ops {
		err = op(idxOps)
		if err != nil {
			return
		}
	}

	switch idxOps.format {
	case types.DockerManifestList:
		return imgutil.Index{
			ImageIndex: &docker.DockerIndex,
			Options: imgutil.IndexOptions{
				KeyChain:         idxOps.keychain,
				XdgPath:          idxOps.xdgPath,
				Reponame:         idxOps.repoName,
				InsecureRegistry: idxOps.insecure,
			},
		}, nil
	case types.OCIImageIndex:
		return imgutil.Index{
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
		}, nil
	default:
		return index, errors.New("unsupported index format")
	}
}
