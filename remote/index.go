package remote

import (
	"fmt"

	"github.com/buildpacks/imgutil"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
)

type ImageIndex struct {
	keychain         authn.Keychain
	repoName         string
	index            v1.ImageIndex
	registrySettings map[string]registrySetting
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
	// if cfg.Architecture == "" {
	// 	return fmt.Errorf("missing Architecture for image %q", repoName)
	// }

	platform := v1.Platform{}
	platform.Architecture = cfg.Architecture
	platform.OS = cfg.OS

	desc.Descriptor.Platform = &platform

	i.index = mutate.AppendManifests(i.index, mutate.IndexAddendum{Add: img, Descriptor: desc.Descriptor})

	return nil
}

func (i *ImageIndex) Remove(repoName string) error {
	ref, err := name.ParseReference(repoName)
	if err != nil {
		panic(err)
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		panic(err)
	}

	i.index = mutate.RemoveManifests(i.index, match.Digests(desc.Digest))

	return nil
}

func (i *ImageIndex) Save(additionalNames ...string) error {
	return i.SaveAs(i.Name(), additionalNames...)
}

func (i *ImageIndex) SaveAs(name string, additionalNames ...string) error {
	allNames := append([]string{name}, additionalNames...)

	var diagnostics []imgutil.SaveIndexDiagnostic
	for _, n := range allNames {
		if err := i.doSave(n); err != nil {
			diagnostics = append(diagnostics, imgutil.SaveIndexDiagnostic{ImageIndexName: n, Cause: err})
		}
	}
	if len(diagnostics) > 0 {
		return imgutil.SaveIndexError{Errors: diagnostics}
	}

	return nil

}

func (i *ImageIndex) doSave(indexName string) error {
	reg := getRegistry(i.repoName, i.registrySettings)
	ref, auth, err := referenceForRepoName(i.keychain, indexName, reg.insecure)
	if err != nil {
		return err
	}
	return remote.WriteIndex(ref, i.index, remote.WithAuth(auth))
}

func (i *ImageIndex) ManifestSize() (int64, error) {
	return i.index.Size()
}

func (i *ImageIndex) Name() string {
	return i.repoName
}
