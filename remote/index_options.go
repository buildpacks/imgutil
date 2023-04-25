package remote

import (
	"github.com/buildpacks/imgutil"
)

type ImageIndexOption func(*indexOptions) error

type indexOptions struct {
	mediaTypes imgutil.MediaTypes
}

// WithIndexMediaTypes loads an existing index as a source.
// If mediatype is not found ignore.
func WithIndexMediaTypes(requested imgutil.MediaTypes) ImageIndexOption {
	return func(opts *indexOptions) error {
		opts.mediaTypes = requested
		return nil
	}
}
