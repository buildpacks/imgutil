package imgutil

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type ImageOption func(*ImageOptions)

type ImageOptions struct {
	BaseImageRepoName     string
	PreviousImageRepoName string
	Config                *v1.Config
	CreatedAt             time.Time
	MediaTypes            MediaTypes
	Platform              Platform
	PreserveHistory       bool
	LayoutOptions
	RemoteOptions

	// These options must be specified in each implementation's image constructor
	BaseImage     v1.Image
	PreviousImage v1.Image
}

type LayoutOptions struct {
	PreserveDigest bool
	WithoutLayers  bool
}

type RemoteOptions struct {
	RegistrySettings    map[string]RegistrySetting
	AddEmptyLayerOnSave bool
}

type RegistrySetting struct {
	Insecure bool
}

// FromBaseImage loads the provided image as the manifest, config, and layers for the working image.
// If the image is not found, it does nothing.
func FromBaseImage(name string) func(*ImageOptions) {
	return func(o *ImageOptions) {
		o.BaseImageRepoName = name
	}
}

// WithConfig lets a caller provided a `config` object for the working image.
func WithConfig(c *v1.Config) func(*ImageOptions) {
	return func(o *ImageOptions) {
		o.Config = c
	}
}

// WithCreatedAt lets a caller set the "created at" timestamp for the working image when saved.
// If not provided, the default is NormalizedDateTime.
func WithCreatedAt(t time.Time) func(*ImageOptions) {
	return func(o *ImageOptions) {
		o.CreatedAt = t
	}
}

// WithDefaultPlatform provides the default Architecture/OS/OSVersion if no base image is provided,
// or if the provided image inputs (base and previous) are manifest lists.
func WithDefaultPlatform(p Platform) func(*ImageOptions) {
	return func(o *ImageOptions) {
		o.Platform = p
	}
}

// WithHistory if provided will configure the image to preserve history when saved
// (including any history from the base image if valid).
func WithHistory() func(*ImageOptions) {
	return func(o *ImageOptions) {
		o.PreserveHistory = true
	}
}

// WithMediaTypes lets a caller set the desired media types for the manifest and config (including layers referenced in the manifest)
// to be either OCI media types or Docker media types.
func WithMediaTypes(m MediaTypes) func(*ImageOptions) {
	return func(o *ImageOptions) {
		o.MediaTypes = m
	}
}

// WithPreviousImage loads an existing image as the source for reusable layers.
// Use with ReuseLayer().
// If the image is not found, it does nothing.
func WithPreviousImage(name string) func(*ImageOptions) {
	return func(o *ImageOptions) {
		o.PreviousImageRepoName = name
	}
}

type IndexOption func(options *IndexOptions) error

type PushOption func(*IndexPushOptions) error

type AddOption func(*IndexAddOptions) error

type IndexAddOptions struct {
	All                          bool
	Local                        bool
	OS, Arch, Variant, OSVersion string
	Features, OSFeatures         []string
	Annotations                  map[string]string
	Image                        EditableImage
}

type IndexPushOptions struct {
	IndexFormatOptions
	IndexRemoteOptions
	Purge bool
	Tags  []string // Tags with which the index should be pushed to registry
}

type IndexFormatOptions struct {
	Format types.MediaType // The Format the Index should be. One of Docker or OCI
}

type IndexRemoteOptions struct {
	Insecure bool
}

type IndexOptions struct {
	XdgPath                string
	BaseImageIndexRepoName string
	KeyChain               authn.Keychain
	IndexFormatOptions
	IndexRemoteOptions

	// These options must be specified in each implementation's image index constructor
	BaseIndex v1.ImageIndex
}

// IndexOptions

// FromBaseImageIndex loads the ImageIndex at the provided path for the working image index.
// If the index is not found, it does nothing.
func FromBaseImageIndex(name string) func(*IndexOptions) error {
	return func(o *IndexOptions) error {
		o.BaseImageIndexRepoName = name
		return nil
	}
}

// FromBaseImageIndexInstance loads the provided image index for the working image index.
// If the index is not found, it does nothing.
func FromBaseImageIndexInstance(index v1.ImageIndex) func(options *IndexOptions) error {
	return func(o *IndexOptions) error {
		o.BaseIndex = index
		return nil
	}
}

// WithKeychain fetches Index from registry with keychain
func WithKeychain(keychain authn.Keychain) func(options *IndexOptions) error {
	return func(o *IndexOptions) error {
		o.KeyChain = keychain
		return nil
	}
}

// WithXDGRuntimePath Saves the Index to the '`xdgPath`/manifests'
func WithXDGRuntimePath(xdgPath string) func(options *IndexOptions) error {
	return func(o *IndexOptions) error {
		o.XdgPath = xdgPath
		return nil
	}
}

// PullInsecure If true, pulls images from insecure registry
func PullInsecure() func(options *IndexOptions) error {
	return func(o *IndexOptions) error {
		o.Insecure = true
		return nil
	}
}

// WithFormat Create the image index with the following format
func WithFormat(format types.MediaType) func(options *IndexOptions) error {
	return func(o *IndexOptions) error {
		o.Format = format
		return nil
	}
}

// IndexAddOptions

// Add all images within the index
func WithAll(all bool) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.All = all
		return nil
	}
}

// Add a single image from index with given OS
func WithOS(os string) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.OS = os
		return nil
	}
}

// Add a Local image to Index
func WithLocalImage(image EditableImage) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.Local = true
		a.Image = image
		return nil
	}
}

// Add a single image from index with given Architecture
func WithArchitecture(arch string) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.Arch = arch
		return nil
	}
}

// Add a single image from index with given Variant
func WithVariant(variant string) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.Variant = variant
		return nil
	}
}

// Add a single image from index with given OSVersion
func WithOSVersion(osVersion string) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.OSVersion = osVersion
		return nil
	}
}

// Add a single image from index with given Features
func WithFeatures(features []string) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.Features = features
		return nil
	}
}

// Add a single image from index with given OSFeatures
func WithOSFeatures(osFeatures []string) func(options *IndexAddOptions) error {
	return func(a *IndexAddOptions) error {
		a.OSFeatures = osFeatures
		return nil
	}
}

// IndexPushOptions

// If true, Deletes index from local filesystem after pushing to registry
func WithPurge(purge bool) func(options *IndexPushOptions) error {
	return func(a *IndexPushOptions) error {
		a.Purge = purge
		return nil
	}
}

// Push the Index with given format
func WithTags(tags ...string) func(options *IndexPushOptions) error {
	return func(a *IndexPushOptions) error {
		a.Tags = tags
		return nil
	}
}

// Push index to Insecure Registry
func WithInsecure(insecure bool) func(options *IndexPushOptions) error {
	return func(a *IndexPushOptions) error {
		a.Insecure = insecure
		return nil
	}
}

// Push the Index with given format
func UsingFormat(format types.MediaType) func(options *IndexPushOptions) error {
	return func(a *IndexPushOptions) error {
		if !format.IsIndex() {
			return fmt.Errorf("unsupported media type encountered in image: '%s'", format)
		}
		a.Format = format
		return nil
	}
}

func GetTransport(insecure bool) http.RoundTripper {
	if insecure {
		return &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // #nosec G402
			},
		}
	}

	return http.DefaultTransport
}
