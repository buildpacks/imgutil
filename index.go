package imgutil

import (
	"github.com/google/go-containerregistry/pkg/name"
)

// ImageIndex an Interface with list of Methods required for creation and manipulation of v1.IndexManifest
type ImageIndex interface {
	// getters

	OS(digest name.Digest) (os string, err error)
	Architecture(digest name.Digest) (arch string, err error)
	Variant(digest name.Digest) (osVariant string, err error)
	OSVersion(digest name.Digest) (osVersion string, err error)
	Features(digest name.Digest) (features []string, err error)
	OSFeatures(digest name.Digest) (osFeatures []string, err error)
	Annotations(digest name.Digest) (annotations map[string]string, err error)
	URLs(digest name.Digest) (urls []string, err error)

	// setters

	SetOS(digest name.Digest, os string) error
	SetArchitecture(digest name.Digest, arch string) error
	SetVariant(digest name.Digest, osVariant string) error
	SetOSVersion(digest name.Digest, osVersion string) error
	SetFeatures(digest name.Digest, features []string) error
	SetOSFeatures(digest name.Digest, osFeatures []string) error
	SetAnnotations(digest name.Digest, annotations map[string]string) error
	SetURLs(digest name.Digest, urls []string) error

	// misc

	// TODO change the name.Reference and expose String?
	Add(ref name.Reference, ops ...func(options *IndexAddOptions) error) error
	Save() error
	Push(ops ...func(options *IndexPushOptions) error) error
	Inspect() (string, error)
	Remove(ref name.Reference) error
	Delete() error
}
