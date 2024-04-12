package layout

import (
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

// FromBaseImageInstance loads the provided image as the manifest, config, and layers for the working image.
// If the image is not found, it does nothing.
func FromBaseImageInstance(image v1.Image) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.BaseImage = image
	}
}

// WithoutLayersWhenSaved (layout only) if provided will cause the image to be written without layers in the `blobs` directory.
func WithoutLayersWhenSaved() func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.WithoutLayers = true
	}
}

// FIXME: the following functions are defined in this package for backwards compatibility,
// and should eventually be deprecated.

// FromBaseImagePath loads the image at the provided path as the manifest, config, and layers for the working image.
// If the image is not found, it does nothing.
func FromBaseImagePath(name string) func(*imgutil.ImageOptions) {
	return imgutil.FromBaseImage(name)
}

func WithConfig(c *v1.Config) func(*imgutil.ImageOptions) {
	return imgutil.WithConfig(c)
}

func WithCreatedAt(t time.Time) func(*imgutil.ImageOptions) {
	return imgutil.WithCreatedAt(t)
}

func WithDefaultPlatform(p imgutil.Platform) func(*imgutil.ImageOptions) {
	return imgutil.WithDefaultPlatform(p)
}

func WithHistory() func(*imgutil.ImageOptions) {
	return imgutil.WithHistory()
}

func WithMediaTypes(m imgutil.MediaTypes) func(*imgutil.ImageOptions) {
	return imgutil.WithMediaTypes(m)
}

func WithPreviousImage(name string) func(*imgutil.ImageOptions) {
	return imgutil.WithPreviousImage(name)
}

// Image Index Stuff!!!

type Option func(options *imgutil.IndexOptions) error
type PushOption func(*imgutil.IndexPushOptions) error
type AddOption func(*imgutil.IndexAddOptions) error

// FromBaseImageIndexInstance loads the provided image index for the working image index.
// If the index is not found, it does nothing.
func FromBaseImageIndexInstance(index v1.ImageIndex) func(options *imgutil.IndexOptions) error {
	return func(o *imgutil.IndexOptions) error {
		o.BaseIndex = index
		return nil
	}
}

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
