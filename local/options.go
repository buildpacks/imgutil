package local

import (
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

// FromBaseImage loads the provided image as the manifest, config, and layers for the working image.
// If the image is not found, it does nothing.
func FromBaseImage(name string) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.BaseImageRepoName = name
	}
}

// WithConfig lets a caller provided a `config` object for the working image.
func WithConfig(c *v1.Config) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.Config = c
	}
}

// WithCreatedAt lets a caller set the "created at" timestamp for the working image when saved.
// If not provided, the default is imgutil.NormalizedDateTime.
func WithCreatedAt(t time.Time) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.CreatedAt = t
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

// Image Index Stuff!!!

type Option func(options *imgutil.IndexOptions) error
type PushOption func(*imgutil.IndexPushOptions) error
type AddOption func(*imgutil.IndexAddOptions) error

// WithKeychain fetches Index from registry with keychain
func WithKeychain(keychain authn.Keychain) Option {
	return func(o *imgutil.IndexOptions) error {
		o.KeyChain = keychain
		return nil
	}
}

// WithXDGRuntimePath Saves the Index to the '`xdgPath`/manifests'
func WithXDGRuntimePath(xdgPath string) Option {
	return func(o *imgutil.IndexOptions) error {
		o.XdgPath = xdgPath
		return nil
	}
}

// PullInsecure If true, pulls images from insecure registry
func PullInsecure() Option {
	return func(o *imgutil.IndexOptions) error {
		o.Insecure = true
		return nil
	}
}

// Push index to Insecure Registry
func WithInsecure(insecure bool) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		a.Insecure = insecure
		return nil
	}
}

// Push the Index with given format
func UsingFormat(format types.MediaType) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		if !format.IsIndex() {
			return fmt.Errorf("unsupported media type encountered in image: '%s'", format)
		}
		a.Format = format
		return nil
	}
}

// UsingFormat Create the image index with the following format
func WithFormat(format types.MediaType) Option {
	return func(o *imgutil.IndexOptions) error {
		o.Format = format
		return nil
	}
}

// Others

// Add all images within the index
func WithAll(all bool) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.All = all
		return nil
	}
}

// Add a single image from index with given OS
func WithOS(os string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.OS = os
		return nil
	}
}

// Add a Local image to Index
func WithLocalImage(image imgutil.EditableImage) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Local = true
		a.Image = image
		return nil
	}
}

// Add a single image from index with given Architecture
func WithArchitecture(arch string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Arch = arch
		return nil
	}
}

// Add a single image from index with given Variant
func WithVariant(variant string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Variant = variant
		return nil
	}
}

// Add a single image from index with given OSVersion
func WithOSVersion(osVersion string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.OSVersion = osVersion
		return nil
	}
}

// Add a single image from index with given Features
func WithFeatures(features []string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Features = features
		return nil
	}
}

// Add a single image from index with given OSFeatures
func WithOSFeatures(osFeatures []string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.OSFeatures = osFeatures
		return nil
	}
}

// If true, Deletes index from local filesystem after pushing to registry
func WithPurge(purge bool) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		a.Purge = purge
		return nil
	}
}

// Push the Index with given format
func WithTags(tags ...string) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		a.Tags = tags
		return nil
	}
}
