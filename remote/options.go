package remote

import (
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil"
)

type ImageOption func(o *imgutil.ImageOptions)

// AddEmptyLayerOnSave adds an empty layer before saving if the image has no layers at all.
// This option is useful when exporting to registries that do not allow saving an image without layers,
// for example: gcr.io.
func AddEmptyLayerOnSave() ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.AddEmptyLayerOnSave = true
	}
}

// FromBaseImage loads the provided image as the manifest, config, and layers for the working image.
// If the image is not found, it does nothing.
func FromBaseImage(name string) ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.BaseImageRepoName = name
	}
}

// WithConfig lets a caller provided a `config` object for the working image.
func WithConfig(c *v1.Config) ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.Config = c
	}
}

// WithCreatedAt lets a caller set the "created at" timestamp for the working image when saved.
// If not provided, the default is imgutil.NormalizedDateTime.
func WithCreatedAt(t time.Time) ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.CreatedAt = t
	}
}

// WithDefaultPlatform provides the default Architecture/OS/OSVersion if no base image is provided,
// or if the provided image inputs (base and previous) are manifest lists.
func WithDefaultPlatform(p imgutil.Platform) ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.Platform = p
	}
}

// WithHistory if provided will configure the image to preserve history when saved
// (including any history from the base image if valid).
func WithHistory() ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.PreserveHistory = true
	}
}

// WithMediaTypes lets a caller set the desired media types for the manifest and config (including layers referenced in the manifest)
// to be either OCI media types or Docker media types.
func WithMediaTypes(m imgutil.MediaTypes) ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.MediaTypes = m
	}
}

// WithPreviousImage loads an existing image as the source for reusable layers.
// Use with ReuseLayer().
// If the image is not found, it does nothing.
func WithPreviousImage(name string) ImageOption {
	return func(o *imgutil.ImageOptions) {
		o.PreviousImageRepoName = name
	}
}

// WithRegistrySetting (remote only) registers options to use
// when accessing images in a registry
// in order to construct the image.
// The referenced images could include the base image, a previous image, or the image itself.
// The insecure parameter allows image references to be fetched without TLS.
func WithRegistrySetting(repository string, insecure bool) ImageOption {
	return func(o *imgutil.ImageOptions) {
		if o.RegistrySettings == nil {
			o.RegistrySettings = make(map[string]imgutil.RegistrySetting)
		}
		o.RegistrySettings[repository] = imgutil.RegistrySetting{
			Insecure: insecure,
		}
	}
}
