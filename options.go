package imgutil

import (
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type ImageOptions struct {
	BaseImageRepoName     string
	Config                *v1.Config
	CreatedAt             time.Time
	Platform              Platform
	PreserveHistory       bool
	PreviousImageRepoName string
	MediaTypes            MediaTypes

	// These options are specified in each implementation's image constructor
	BaseImage     v1.Image
	PreviousImage v1.Image
}
