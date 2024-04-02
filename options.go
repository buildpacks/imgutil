package imgutil

import (
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
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
