package remote

import (
	"time"

	"github.com/buildpacks/imgutil"
)

type ImageOption func(*options) error

type options struct {
	platform            imgutil.Platform
	baseImageRepoName   string
	prevImageRepoName   string
	createdAt           time.Time
	addEmptyLayerOnSave bool
	registrySettings    map[string]registrySetting
}

// AddEmptyLayerOnSave adds an empty layer before saving if the image has no layer at all.
// This option is useful when exporting to registries that do not allow saving an image without layers,
// for example: gcr.io
func AddEmptyLayerOnSave() ImageOption {
	return func(opts *options) error {
		opts.addEmptyLayerOnSave = true
		return nil
	}
}

// FromBaseImage loads an existing image as the config and layers for the new image.
// Ignored if image is not found.
func FromBaseImage(imageName string) ImageOption {
	return func(opts *options) error {
		opts.baseImageRepoName = imageName
		return nil
	}
}

// WithCreatedAt lets a caller set the created at timestamp for the image.
// Defaults for a new image is imgutil.NormalizedDateTime
func WithCreatedAt(createdAt time.Time) ImageOption {
	return func(opts *options) error {
		opts.createdAt = createdAt
		return nil
	}
}

// WithDefaultPlatform provides Architecture/OS/OSVersion defaults for the new image.
// Defaults for a new image are ignored when FromBaseImage returns an image.
// FromBaseImage and WithPreviousImage will use the platform to choose an image from a manifest list.
func WithDefaultPlatform(platform imgutil.Platform) ImageOption {
	return func(opts *options) error {
		opts.platform = platform
		return nil
	}
}

// WithPreviousImage loads an existing image as a source for reusable layers.
// Use with ReuseLayer().
// Ignored if image is not found.
func WithPreviousImage(imageName string) ImageOption {
	return func(opts *options) error {
		opts.prevImageRepoName = imageName
		return nil
	}
}

// WithRegistrySetting registers options to use when accessing images in a registry in order to construct
// the image. The referenced images could include the base image, a previous image, or the image itself.
func WithRegistrySetting(repository string, insecure, insecureSkipVerify bool) ImageOption {
	return func(opts *options) error {
		opts.registrySettings[repository] = registrySetting{
			insecure:           insecure,
			insecureSkipVerify: insecureSkipVerify,
		}
		return nil
	}
}
