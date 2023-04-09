package imgutil

import "github.com/google/go-containerregistry/pkg/v1/types"

type ImageIndex interface {
	Add(repoName string) error
	Remove(repoName string) error
	Save() error
}

func (t MediaTypes) IndexManifestType() types.MediaType {
	switch t {
	case OCITypes:
		return types.OCIImageIndex
	case DockerTypes:
		return types.DockerManifestList
	default:
		return ""
	}
}
