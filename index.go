package imgutil

import (
	"github.com/google/go-containerregistry/pkg/name"
)

// ImageIndex an Interface with list of Methods required for creation and manipulation of v1.IndexManifest
type ImageIndex interface {
	// getters

	Annotations(digest name.Digest) (annotations map[string]string, err error)
	Architecture(digest name.Digest) (arch string, err error)
	Features(digest name.Digest) (features []string, err error)
	OS(digest name.Digest) (os string, err error)
	OSFeatures(digest name.Digest) (osFeatures []string, err error)
	OSVersion(digest name.Digest) (osVersion string, err error)
	Variant(digest name.Digest) (osVariant string, err error)

	// setters

	SetAnnotations(digest name.Digest, annotations map[string]string) error
	SetArchitecture(digest name.Digest, arch string) error
	SetFeatures(digest name.Digest, features []string) error
	SetOS(digest name.Digest, os string) error
	SetOSFeatures(digest name.Digest, osFeatures []string) error
	SetOSVersion(digest name.Digest, osVersion string) error
	SetVariant(digest name.Digest, osVariant string) error

	// misc

	Add(repoName string, ops ...func(options *IndexAddOptions) error) error
	Delete() error
	Inspect() (string, error)
	Push(ops ...func(options *IndexPushOptions) error) error
	Remove(repoName string) error
	Save() error
}
