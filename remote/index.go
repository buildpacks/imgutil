package remote

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

type ImageIndex struct {
	keychain         authn.Keychain
	repoName         string
	index            v1.ImageIndex
	registrySettings map[string]registrySetting
}

// modfiers

// Add appends a new image manifest to the remote ImageIndex/ManifestList.
// We have not implemented nested indexes yet.
// See specification for more info:
// https://github.com/opencontainers/image-spec/blob/0b40f0f367c396cc5a7d6a2e8c8842271d3d3844/image-index.md#image-index-property-descriptions
func (i *ImageIndex) Add(repoName string) error {
	ref, err := name.ParseReference(repoName)
	if err != nil {
		return err
	}

	// Fetch image descriptor from registry
	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return errors.Wrapf(err, "error fetching %s from registry", repoName)
	}

	img, err := desc.Image()
	if err != nil {
		return err
	}

	// Get the image configuration file
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

// Remove method removes the specified manifest from the index
func (i *ImageIndex) Remove(repoName string) error {
	ref, err := name.ParseReference(repoName)
	if err != nil {
		return err
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	i.index = mutate.RemoveManifests(i.index, match.Digests(desc.Digest))

	return nil
}

// Save pushes the ImageIndex to the image reference obtained from index name.
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

	iManifest, err := i.index.IndexManifest()

	// This for loop will check if all the referenced manifests have the plaform information.
	// This is OPTIONAL if the target is plaform independent.
	// Current implementation does not allow to push an index without platform infromation.
	for _, j := range iManifest.Manifests {
		switch j.MediaType {
		case types.OCIManifestSchema1, types.DockerManifestSchema2:
			if j.Platform.Architecture == "" || j.Platform.OS == "" {
				return errors.Errorf("manifest with digest %s is missing either OS or Architecture information to be pushed to a registry", j.Digest)
			}
		}
	}

	return remote.WriteIndex(ref, i.index, remote.WithAuth(auth))
}

func (i *ImageIndex) Name() string {
	return i.repoName
}

// This structure is used to expose methods that we only need for testing.
type ImageIndexTest struct {
	ImageIndex
}

func (i *ImageIndexTest) MediaType() (types.MediaType, error) {
	mediaType, err := i.ImageIndex.index.MediaType()
	if err != nil {
		return "", err
	}

	return mediaType, nil
}
