package index

import (
	"errors"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
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
			ImageIndex: &DockerIndex,
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
