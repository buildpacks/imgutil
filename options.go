package imgutil

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

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

type IndexAddOption func(*AddOptions)
type IndexPushOption func(*PushOptions) error

type AddOptions struct {
	All                          bool
	Local                        bool
	OS, Arch, Variant, OSVersion string
	Features, OSFeatures         []string
	Annotations                  map[string]string
	Image                        EditableImage
}

type PushOptions struct {
	Insecure, Purge bool
	// The Format the Index should be. One of Docker or OCI
	Format types.MediaType
	// Tags with which the index should be pushed to registry
	Tags []string
}

type IndexOptions struct {
	KeyChain          authn.Keychain
	XdgPath, Reponame string
	InsecureRegistry  bool
}

func (o *IndexOptions) Keychain() authn.Keychain {
	return o.KeyChain
}

func (o *IndexOptions) XDGRuntimePath() string {
	return o.XdgPath
}

func (o *IndexOptions) RepoName() string {
	return o.Reponame
}

func (o *IndexOptions) Insecure() bool {
	return o.InsecureRegistry
}

// Add all images within the index
func WithAll(all bool) IndexAddOption {
	return func(a *AddOptions) {
		a.All = all
	}
}

// Add a single image from index with given OS
func WithOS(os string) IndexAddOption {
	return func(a *AddOptions) {
		a.OS = os
	}
}

// Add a Local image to Index
func WithLocalImage(image EditableImage) IndexAddOption {
	return func(a *AddOptions) {
		a.Local = true
		a.Image = image
	}
}

// Add a single image from index with given Architecture
func WithArchitecture(arch string) IndexAddOption {
	return func(a *AddOptions) {
		a.Arch = arch
	}
}

// Add a single image from index with given Variant
func WithVariant(variant string) IndexAddOption {
	return func(a *AddOptions) {
		a.Variant = variant
	}
}

// Add a single image from index with given OSVersion
func WithOSVersion(osVersion string) IndexAddOption {
	return func(a *AddOptions) {
		a.OSVersion = osVersion
	}
}

// Add a single image from index with given Features
func WithFeatures(features []string) IndexAddOption {
	return func(a *AddOptions) {
		a.Features = features
	}
}

// Add a single image from index with given OSFeatures
func WithOSFeatures(osFeatures []string) IndexAddOption {
	return func(a *AddOptions) {
		a.OSFeatures = osFeatures
	}
}

// Add a single image from index with given Annotations
func WithAnnotations(annotations map[string]string) IndexAddOption {
	return func(a *AddOptions) {
		a.Annotations = annotations
	}
}

// Push index to Insecure Registry
func WithInsecure(insecure bool) IndexPushOption {
	return func(a *PushOptions) error {
		a.Insecure = insecure
		return nil
	}
}

// If true, Deletes index from local filesystem after pushing to registry
func WithPurge(purge bool) IndexPushOption {
	return func(a *PushOptions) error {
		a.Purge = purge
		return nil
	}
}

// Push the Index with given format
func WithFormat(format types.MediaType) IndexPushOption {
	return func(a *PushOptions) error {
		if !format.IsIndex() {
			return ErrUnknownMediaType(format)
		}
		a.Format = format
		return nil
	}
}

// Push the Index with given format
func WithTags(tags ...string) IndexPushOption {
	return func(a *PushOptions) error {
		a.Tags = tags
		return nil
	}
}

func GetTransport(insecure bool) http.RoundTripper {
	// #nosec G402
	if insecure {
		return &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return http.DefaultTransport
}
