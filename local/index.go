package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type ImageIndex struct {
	repoName string
	path     string
	index    v1.ImageIndex
}

// Add appends a new image manifest to the local ImageIndex/ManifestList.
// We have not implemented nested indexes yet.
// See specification for more info:
// https://github.com/opencontainers/image-spec/blob/0b40f0f367c396cc5a7d6a2e8c8842271d3d3844/image-index.md#image-index-property-descriptions
func (i *ImageIndex) Add(repoName string) error {
	ref, err := name.ParseReference(repoName)
	if err != nil {
		return err
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	img, err := desc.Image()
	if err != nil {
		return err
	}

	cfg, err := img.ConfigFile()

	if err != nil {
		return errors.Wrapf(err, "getting config file for image %q", repoName)
	}
	if cfg == nil {
		return fmt.Errorf("missing config for image %q", repoName)
	}

	platform := v1.Platform{}
	platform.Architecture = cfg.Architecture
	platform.OS = cfg.OS

	desc.Descriptor.Platform = &platform

	indexRef, err := name.ParseReference(i.repoName)
	if err != nil {
		return err
	}

	// Check if the image is in the same repository as the index
	// If it is in a different repository then copy the image to
	// the same repository as the index
	if ref.Context().Name() != indexRef.Context().Name() {
		imgRefName := indexRef.Context().Name() + "@" + desc.Digest.Algorithm + ":" + desc.Digest.Hex
		imgRef, err := name.ParseReference(imgRefName)
		if err != nil {
			return err
		}

		err = remote.Write(imgRef, img, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return errors.Wrapf(err, "failed to copy image '%s' to index repository", imgRef.Name())
		}
	}

	i.index = mutate.AppendManifests(i.index, mutate.IndexAddendum{Add: img, Descriptor: desc.Descriptor})

	return nil
}

// Remove method removes the specified manifest from the local index
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

// Delete method removes the specified index from the local storage
func (i *ImageIndex) Delete(additionalNames ...string) error {
	_, err := name.ParseReference(i.repoName)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(i.path, makeFileSafeName(i.repoName))
	err = os.Remove(manifestPath)
	if err != nil {
		return err
	}

	return nil
}

// Save stores the ImageIndex manifest information in a plain text in the ined file in JSON format.
func (i *ImageIndex) Save(additionalNames ...string) error {
	indexManifest, err := i.index.IndexManifest()
	if err != nil {
		return err
	}

	rawManifest, err := json.MarshalIndent(indexManifest, "", "    ")
	if err != nil {
		return err
	}

	manifestDir := filepath.Join(i.path, makeFileSafeName(i.repoName))

	err = os.WriteFile(manifestDir, rawManifest, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

// Change a reference name string into a valid file name
// Ex: cnbs/sample-package:hello-multiarch-universe
// to cnbs_sample-package-hello-multiarch-universe
func makeFileSafeName(ref string) string {
	fileName := strings.ReplaceAll(ref, ":", "-")
	return strings.ReplaceAll(fileName, "/", "_")
}

func (i *ImageIndex) Name() string {
	return i.repoName
}

// Fields which are allowed to be annotated in a local index
type AnnotateFields struct {
	Architecture string
	OS           string
	Variant      string
}

// AnnotateManifest changes the fields of the local index which
// are not empty string in the provided AnnotateField structure.
func (i *ImageIndex) AnnotateManifest(manifestName string, opts AnnotateFields) error {
	path := filepath.Join(i.path, makeFileSafeName(i.repoName))

	manifest, err := i.index.IndexManifest()
	if err != nil {
		return err
	}

	ref, err := name.ParseReference(manifestName)
	if err != nil {
		return err
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	for i, iDesc := range manifest.Manifests {
		if iDesc.Digest.String() == desc.Digest.String() {
			if opts.Architecture != "" {
				manifest.Manifests[i].Platform.Architecture = opts.Architecture
			}

			if opts.OS != "" {
				manifest.Manifests[i].Platform.OS = opts.OS
			}

			if opts.Variant != "" {
				manifest.Manifests[i].Platform.Variant = opts.Variant
			}

			data, err := json.Marshal(manifest)
			if err != nil {
				return err
			}

			err = os.WriteFile(path, data, os.ModePerm)
			if err != nil {
				return err
			}

			return nil
		}
	}

	return errors.Errorf("Manifest %s not found", manifestName)
}

// GetIndexManifest will look for a file the given index in the specified path and
// if found it will return a v1.IndexManifest.
// It is assumed that the local index file name is derived using makeFileSafeName()
func GetIndexManifest(repoName string, path string) (v1.IndexManifest, error) {
	var manifest v1.IndexManifest

	_, err := name.ParseReference(repoName)
	if err != nil {
		return manifest, err
	}

	manifestDir := filepath.Join(path, makeFileSafeName(repoName))

	jsonFile, err := os.ReadFile(manifestDir)
	if err != nil {
		return manifest, errors.Wrapf(err, "Reading local index %q in path %q", repoName, path)
	}

	err = json.Unmarshal(jsonFile, &manifest)
	if err != nil {
		return manifest, errors.Wrapf(err, "Decoding local index %q", repoName)
	}

	return manifest, nil
}
