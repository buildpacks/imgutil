package remote

import (
	"fmt"

	"github.com/buildpacks/imgutil/layout"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
)

type ImageIndex struct {
	keychain authn.Keychain
	repoName string
	index    v1.ImageIndex
}

func (i *ImageIndex) Add(repoName string) error {
	ref, err := name.ParseReference(repoName)
	if err != nil {
		panic(err)
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		panic(err)
	}

	img, err := desc.Image()
	if err != nil {
		panic(err)
	}

	cfg, err := img.ConfigFile()

	if err != nil {
		return errors.Wrapf(err, "getting config file for image %q", repoName)
	}
	if cfg == nil {
		return fmt.Errorf("missing config for image %q", repoName)
	}
	if cfg.OS == "" {
		return fmt.Errorf("missing OS for image %q", repoName)
	}

	platform := v1.Platform{}
	platform.Architecture = cfg.Architecture
	platform.OS = cfg.OS

	desc.Descriptor.Platform = &platform

	i.index = mutate.AppendManifests(i.index, mutate.IndexAddendum{Add: img, Descriptor: desc.Descriptor})

	return nil
}

// func (i *ImageIndex) Remove(repoName string) error

func (i *ImageIndex) Save(path string) error {
	// write the index on disk, for example
	layout.Write(path, i.index)

	return nil
}
