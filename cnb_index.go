package imgutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/pkg/errors"
)

var (
	ErrManifestUndefined = errors.New("encountered unexpected error while parsing image: manifest or index manifest is nil")
	ErrUnknownMediaType  = func(format types.MediaType) error {
		return fmt.Errorf("unsupported media type encountered in image: '%s'", format)
	}
)

type CNBIndex struct {
	// required
	v1.ImageIndex // the working image index
	// local options
	XdgPath string
	// push options
	KeyChain authn.Keychain
	RepoName string
}

func (h *CNBIndex) getConfigFileFrom(digest name.Digest) (v1.ConfigFile, error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return v1.ConfigFile{}, err
	}
	image, err := h.Image(hash)
	if err != nil {
		return v1.ConfigFile{}, err
	}
	configFile, err := GetConfigFile(image)
	if err != nil {
		return v1.ConfigFile{}, err
	}
	return *configFile, nil
}

func (h *CNBIndex) getManifestFileFrom(digest name.Digest) (v1.Manifest, error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return v1.Manifest{}, err
	}
	image, err := h.Image(hash)
	if err != nil {
		return v1.Manifest{}, err
	}
	manifestFile, err := GetManifest(image)
	if err != nil {
		return v1.Manifest{}, err
	}
	return *manifestFile, nil
}

// OS returns `OS` of an existing Image.
func (h *CNBIndex) OS(digest name.Digest) (os string, err error) {
	configFile, err := h.getConfigFileFrom(digest)
	if err != nil {
		return "", err
	}
	return configFile.OS, nil
}

// Architecture return the Architecture of an Image/Index based on given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *CNBIndex) Architecture(digest name.Digest) (arch string, err error) {
	configFile, err := h.getConfigFileFrom(digest)
	if err != nil {
		return "", err
	}
	return configFile.Architecture, nil
}

// Variant return the `Variant` of an Image.
// Returns an error if no Image/Index found with given Digest.
func (h *CNBIndex) Variant(digest name.Digest) (osVariant string, err error) {
	configFile, err := h.getConfigFileFrom(digest)
	if err != nil {
		return "", err
	}
	return configFile.Variant, nil
}

// OSVersion returns the `OSVersion` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *CNBIndex) OSVersion(digest name.Digest) (osVersion string, err error) {
	configFile, err := h.getConfigFileFrom(digest)
	if err != nil {
		return "", err
	}
	return configFile.OSVersion, nil
}

// OSFeatures returns the `OSFeatures` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *CNBIndex) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	configFile, err := h.getConfigFileFrom(digest)
	if err != nil {
		return nil, err
	}
	return configFile.OSFeatures, nil
}

// Annotations return the `Annotations` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
// For Docker images and Indexes it returns an error.
func (h *CNBIndex) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	manifestFile, err := h.getManifestFileFrom(digest)
	if err != nil {
		return nil, err
	}
	return manifestFile.Annotations, nil
}

// setters

func (h *CNBIndex) SetAnnotations(digest name.Digest, annotations map[string]string) (err error) {
	return h.mutateExistingImage(digest, func(image v1.Image) (v1.Image, error) {
		partial := mutate.Annotations(image, annotations)
		annotatedImage, ok := partial.(v1.Image)
		if !ok {
			return nil, fmt.Errorf("failed to annotate image")
		}
		return annotatedImage, nil
	})
}

func (h *CNBIndex) SetArchitecture(digest name.Digest, arch string) (err error) {
	return h.mutateExistingImage(digest, func(image v1.Image) (v1.Image, error) {
		configFile, err := image.ConfigFile()
		if err != nil {
			return nil, err
		}
		configFile.Architecture = arch
		return mutate.ConfigFile(image, configFile)
	})
}

func (h *CNBIndex) SetOS(digest name.Digest, os string) (err error) {
	return h.mutateExistingImage(digest, func(image v1.Image) (v1.Image, error) {
		configFile, err := image.ConfigFile()
		if err != nil {
			return nil, err
		}
		configFile.OS = os
		return mutate.ConfigFile(image, configFile)
	})
}

func (h *CNBIndex) SetVariant(digest name.Digest, osVariant string) (err error) {
	return h.mutateExistingImage(digest, func(image v1.Image) (v1.Image, error) {
		configFile, err := image.ConfigFile()
		if err != nil {
			return nil, err
		}
		configFile.Variant = osVariant
		return mutate.ConfigFile(image, configFile)
	})
}

func (h *CNBIndex) mutateExistingImage(digest name.Digest, withFunc func(image v1.Image) (v1.Image, error)) (err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}
	image, err := h.Image(hash)
	if err != nil {
		return err
	}
	if err = h.RemoveManifest(digest); err != nil {
		return err
	}
	newImage, err := withFunc(image)
	if err != nil {
		return err
	}
	h.AddManifest(newImage)
	return nil
}

func (h *CNBIndex) Image(hash v1.Hash) (v1.Image, error) {
	index, err := h.IndexManifest()
	if err != nil {
		return nil, err
	}
	if !indexContains(index.Manifests, hash) {
		return nil, fmt.Errorf("failed to find image with digest %s in index", hash.String())
	}
	return h.ImageIndex.Image(hash)
}

func indexContains(manifests []v1.Descriptor, hash v1.Hash) bool {
	for _, m := range manifests {
		if m.Digest.String() == hash.String() {
			return true
		}
	}
	return false
}

// AddManifest adds an image to the index.
func (h *CNBIndex) AddManifest(image v1.Image) {
	h.ImageIndex = mutate.AppendManifests(h.ImageIndex, mutate.IndexAddendum{
		Add: image,
	})
}

// SaveDir will locally save the index.
func (h *CNBIndex) SaveDir() error {
	layoutPath := filepath.Join(h.XdgPath, MakeFileSafeName(h.RepoName)) // FIXME: do we create an OCI-layout compatible directory structure?
	var (
		path layout.Path
		err  error
	)

	if _, err = os.Stat(layoutPath); !os.IsNotExist(err) {
		// We need to always init an empty index when saving
		if err = os.RemoveAll(layoutPath); err != nil {
			return err
		}
	}

	indexType, err := h.ImageIndex.MediaType()
	if err != nil {
		return err
	}
	if path, err = newEmptyLayoutPath(indexType, layoutPath); err != nil {
		return err
	}

	var errs SaveError
	index, err := h.ImageIndex.IndexManifest()
	if err != nil {
		return err
	}
	for _, desc := range index.Manifests {
		appendManifest(desc, path, &errs)
	}
	if len(errs.Errors) != 0 {
		return errs
	}
	return nil
}

func appendManifest(desc v1.Descriptor, path layout.Path, errs *SaveError) {
	if err := path.RemoveDescriptors(match.Digests(desc.Digest)); err != nil {
		errs.Errors = append(errs.Errors, SaveDiagnostic{
			Cause: err,
		})
	}
	if err := path.AppendDescriptor(desc); err != nil {
		errs.Errors = append(errs.Errors, SaveDiagnostic{
			Cause: err,
		})
	}
}

func newEmptyLayoutPath(indexType types.MediaType, path string) (layout.Path, error) {
	if indexType == types.OCIImageIndex {
		return layout.Write(path, empty.Index)
	}
	return layout.Write(path, NewEmptyDockerIndex())
}

// Push Publishes ImageIndex to the registry assuming every image it referes exists in registry.
//
// It will only push the IndexManifest to registry.
func (h *CNBIndex) Push(ops ...func(*IndexOptions) error) error {
	var pushOps = &IndexOptions{}
	for _, op := range ops {
		if err := op(pushOps); err != nil {
			return err
		}
	}

	if pushOps.MediaType != "" {
		if !pushOps.MediaType.IsIndex() {
			return ErrUnknownMediaType(pushOps.MediaType)
		}
		existingType, err := h.ImageIndex.MediaType()
		if err != nil {
			return err
		}
		if pushOps.MediaType != existingType {
			h.ImageIndex = mutate.IndexMediaType(h.ImageIndex, pushOps.MediaType)
		}
	}

	ref, err := name.ParseReference(
		h.RepoName,
		name.WeakValidation,
		name.Insecure,
	)
	if err != nil {
		return err
	}

	indexManifest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return err
	}

	var taggableIndex = NewTaggableIndex(indexManifest)
	multiWriteTagables := map[name.Reference]remote.Taggable{
		ref: taggableIndex,
	}
	for _, tag := range pushOps.DestinationTags {
		multiWriteTagables[ref.Context().Tag(tag)] = taggableIndex
	}

	// FIXME: this will only push the index manifest, assuming that all the images it refers to exist in the registry
	err = remote.MultiWrite(
		multiWriteTagables,
		remote.WithAuthFromKeychain(h.KeyChain),
		remote.WithTransport(GetTransport(pushOps.Insecure)),
	)
	if err != nil {
		return err
	}

	if pushOps.Purge {
		return h.DeleteDir()
	}
	return h.SaveDir()
}

// Inspect Displays IndexManifest.
func (h *CNBIndex) Inspect() (string, error) {
	rawManifest, err := h.RawManifest()
	if err != nil {
		return "", err
	}
	return string(rawManifest), nil
}

// RemoveManifest removes an image with a given digest from the index.
func (h *CNBIndex) RemoveManifest(digest name.Digest) (err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}
	h.ImageIndex = mutate.RemoveManifests(h.ImageIndex, match.Digests(hash))
	_, err = h.ImageIndex.Digest() // force compute
	return err
}

// DeleteDir removes the index from the local filesystem if it exists.
func (h *CNBIndex) DeleteDir() error {
	layoutPath := filepath.Join(h.XdgPath, MakeFileSafeName(h.RepoName))
	if _, err := os.Stat(layoutPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(layoutPath)
}

func getIndexManifest(ii v1.ImageIndex) (mfest *v1.IndexManifest, err error) {
	mfest, err = ii.IndexManifest()
	if mfest == nil {
		return mfest, ErrManifestUndefined
	}
	return mfest, err
}
