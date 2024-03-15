package imgutil

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
)

func MutateManifest(i v1.Image, withFunc func(c *v1.Manifest)) (v1.Image, error) {
	// FIXME: put MutateManifest on the interface when `remote` and `layout` packages also support it.
	digest, err := i.Digest()
	if err != nil {
		return nil, err
	}

	mfest, err := getManifest(i)
	if err != nil {
		return nil, err
	}

	config := mfest.Config
	config.Digest = digest
	config.MediaType = mfest.MediaType
	if config.Size, err = partial.Size(i); err != nil {
		return nil, err
	}
	config.Annotations = mfest.Annotations

	p := config.Platform
	if p == nil {
		p = &v1.Platform{}
	}

	config.Platform = p
	mfest.Config = config

	withFunc(mfest)
	if len(mfest.Annotations) != 0 {
		i = mutate.Annotations(i, mfest.Annotations).(v1.Image)
	}

	return mutate.Subject(i, mfest.Config).(v1.Image), err
}
