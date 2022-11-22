package sparse

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil/layout"

	"github.com/buildpacks/imgutil"
)

var _ imgutil.Image = (*Image)(nil)

type Image struct {
	layout.Image
	underlyingImage v1.Image
}

// NewImage returns a new Image saved on disk that can be modified
func NewImage(path string, from v1.Image) (*Image, error) {
	img, err := layout.NewImage(path)
	if err != nil {
		return nil, err
	}

	image := &Image{
		Image:           *img,
		underlyingImage: from,
	}
	return image, nil
}

func (i *Image) Save(additionalNames ...string) error {
	if len(additionalNames) > 1 {
		return errors.Errorf("multiple additional names %v are not allow when OCI layout is used", additionalNames)
	}

	layoutPath, err := layout.Write(i.Name(), empty.Index)
	if err != nil {
		return err
	}

	annotations := map[string]string{}
	if len(additionalNames) == 1 {
		annotations["org.opencontainers.image.ref.name"] = additionalNames[0]
	}

	var diagnostics []imgutil.SaveDiagnostic
	err = layoutPath.AppendImage(i.underlyingImage, layout.WithoutLayers(), layout.WithAnnotations(annotations))
	if err != nil {
		diagnostics = append(diagnostics, imgutil.SaveDiagnostic{ImageName: i.Name(), Cause: err})
	}

	if len(diagnostics) > 0 {
		return imgutil.SaveError{Errors: diagnostics}
	}

	return nil
}
