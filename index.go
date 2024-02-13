package imgutil

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/types"
)

type ImageIndex interface {
	// getters

	Name() string

	// modifiers
	Add(repoName string) error
	Remove(repoName string) error
	Save(additionalNames ...string) error
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

type SaveIndexDiagnostic struct {
	ImageIndexName string
	Cause          error
}

type SaveIndexError struct {
	Errors []SaveIndexDiagnostic
}

func (e SaveIndexError) Error() string {
	var errors []string
	for _, d := range e.Errors {
		errors = append(errors, fmt.Sprintf("[%s: %s]", d.ImageIndexName, d.Cause.Error()))
	}
	return fmt.Sprintf("failed to write image to the following tags: %s", strings.Join(errors, ","))
}
