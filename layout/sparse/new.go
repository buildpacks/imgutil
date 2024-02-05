package sparse

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil/layout"
)

// NewImage returns a new Image saved on disk that can be modified
func NewImage(path string, from v1.Image, ops ...layout.ImageOption) (*layout.Image, error) {
	ops = append([]layout.ImageOption{
		layout.FromBaseImage(from),
		layout.WithoutLayersWhenSaved(),
	}, ops...)
	img, err := layout.NewImage(path, ops...)
	if err != nil {
		return nil, err
	}
	return img, nil
}
