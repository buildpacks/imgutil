package imgutil

import (
	"crypto/tls"
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
	IndexFormatOptions
	IndexRemoteOptions
	KeyChain          authn.Keychain
	XdgPath, Reponame string
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
