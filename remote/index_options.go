package remote

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil"
)

type ImageIndexOption func(*indexOptions) error

type indexOptions struct {
	mediaTypes imgutil.MediaTypes
	manifest   v1.IndexManifest
}

// WithIndexMediaTypes lets a caller set the desired media types for the index manifest
func WithIndexMediaTypes(requested imgutil.MediaTypes) ImageIndexOption {
	return func(opts *indexOptions) error {
		opts.mediaTypes = requested
		return nil
	}
}

// WithManifest uses an existing v1.IndexManifest as a base to create the index
func WithManifest(manifest v1.IndexManifest) ImageIndexOption {
	return func(opts *indexOptions) error {
		opts.manifest = manifest
		return nil
	}
}
