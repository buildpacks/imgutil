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
	PreserveDigest        bool
	PreserveHistory       bool
	WithoutLayers         bool // only relevant for layout images

	// These options are specified in each implementation's image constructor
	BaseImage     v1.Image
	PreviousImage v1.Image
}
