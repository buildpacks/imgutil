package local

import (
	"github.com/buildpacks/imgutil"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type ImageIndexOption func(*indexOptions) error

type indexOptions struct {
	mediaTypes imgutil.MediaTypes
	manifest   v1.IndexManifest
}

// WithIndexMediaTypes loads an existing index as a source.
// If mediatype is not found ignore.
func WithIndexMediaTypes(requested imgutil.MediaTypes) ImageIndexOption {
	return func(opts *indexOptions) error {
		opts.mediaTypes = requested
		return nil
	}
}

func WithManifest(manifest v1.IndexManifest) ImageIndexOption {
	return func(opts *indexOptions) error {
		opts.manifest = manifest
		return nil
	}
}
