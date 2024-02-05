package layout

import (
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil"
)

type ImageOption func(*imgutil.ImageOptions)

// FromBaseImage loads the provided image as the manifest, config, and layers for the working image.
// If the image is not found, it does nothing.
func FromBaseImage(image v1.Image) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.BaseImage = image
	}
}

// FromBaseImagePath (layout only) loads the image at the provided path as the manifest, config, and layers for the working image.
// If the image is not found, it does nothing.
func FromBaseImagePath(name string) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.BaseImageRepoName = name
	}
}

// WithCreatedAt lets a caller set the "created at" timestamp for the working image when saved.
// If not provided, the default is imgutil.NormalizedDateTime.
func WithCreatedAt(t time.Time) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.CreatedAt = t
	}
}

// WithConfig lets a caller provided a `config` object for the working image.
func WithConfig(c *v1.Config) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.Config = c
	}
}

// WithDefaultPlatform provides the default Architecture/OS/OSVersion if no base image is provided,
// or if the provided image inputs (base and previous) are manifest lists.
func WithDefaultPlatform(p imgutil.Platform) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.Platform = p
	}
}

// WithHistory if provided will configure the image to preserve history when saved
// (including any history from the base image if valid).
func WithHistory() func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.PreserveHistory = true
	}
}

// WithMediaTypes lets a caller set the desired media types for the manifest and config (including layers referenced in the manifest)
// to be either OCI media types or Docker media types.
func WithMediaTypes(m imgutil.MediaTypes) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.MediaTypes = m
	}
}

// WithPreviousImage loads an existing image as the source for reusable layers.
// Use with ReuseLayer().
// If the image is not found, it does nothing.
func WithPreviousImage(name string) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.PreviousImageRepoName = name
	}
}

// WithoutLayersWhenSaved (layout only) if provided will cause the image to be written without layers in the `blobs` directory.
func WithoutLayersWhenSaved() func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.WithoutLayers = true
	}
}
