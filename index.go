package imgutil

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/sync/errgroup"

	"github.com/buildpacks/imgutil/docker"
)

// An Interface with list of Methods required for creation and manipulation of v1.IndexManifest
type ImageIndex interface {
	// getters

	OS(digest name.Digest) (os string, err error)
	Architecture(digest name.Digest) (arch string, err error)
	Variant(digest name.Digest) (osVariant string, err error)
	OSVersion(digest name.Digest) (osVersion string, err error)
	Features(digest name.Digest) (features []string, err error)
	OSFeatures(digest name.Digest) (osFeatures []string, err error)
	Annotations(digest name.Digest) (annotations map[string]string, err error)
	URLs(digest name.Digest) (urls []string, err error)

	// setters

	SetOS(digest name.Digest, os string) error
	SetArchitecture(digest name.Digest, arch string) error
	SetVariant(digest name.Digest, osVariant string) error
	SetOSVersion(digest name.Digest, osVersion string) error
	SetFeatures(digest name.Digest, features []string) error
	SetOSFeatures(digest name.Digest, osFeatures []string) error
	SetAnnotations(digest name.Digest, annotations map[string]string) error
	SetURLs(digest name.Digest, urls []string) error

	// misc

	Add(ref name.Reference, ops ...IndexAddOption) error
	Save() error
	Push(ops ...IndexPushOption) error
	Inspect() (string, error)
	Remove(ref name.Reference) error
	Delete() error
}

var (
	ErrOSUndefined                        = errors.New("os is undefined")
	ErrArchUndefined                      = errors.New("architecture is undefined")
	ErrVariantUndefined                   = errors.New("variant is undefined")
	ErrOSVersionUndefined                 = errors.New("osVersion is undefined")
	ErrFeaturesUndefined                  = errors.New("features are undefined")
	ErrOSFeaturesUndefined                = errors.New("os-features are undefined")
	ErrURLsUndefined                      = errors.New("urls are undefined")
	ErrAnnotationsUndefined               = errors.New("annotations are undefined")
	ErrNoImageOrIndexFoundWithGivenDigest = errors.New("no image/index found with the given digest")
	ErrConfigFilePlatformUndefined        = errors.New("platform is undefined in config file")
	ErrManifestUndefined                  = errors.New("manifest is undefined")
	ErrPlatformUndefined                  = errors.New("platform is undefined")
	ErrInvalidPlatform                    = errors.New("invalid platform is provided")
	ErrConfigFileUndefined                = errors.New("config file is undefined")
	ErrIndexNeedToBeSaved                 = errors.New("image index should need to be saved to perform this operation")
	ErrUnknownMediaType                   = errors.New("media type not supported")
	ErrNoImageFoundWithGivenPlatform      = errors.New("no image found with the given platform")
	ErrUnknownHandler                     = errors.New("unknown Handler")
)

var _ ImageIndex = (*IndexHandler)(nil)
var _ ImageIndex = (*ManifestHandler)(nil)

// A Handler implementing ImageIndex.
// It will create and Manipulate ImageIndex along with underlying Images.
//
// Prefer `ManifestHandler` if only IndexManifest should need to be manipulated.
type IndexHandler struct {
	v1.ImageIndex
	Annotate         Annotate
	Options          IndexOptions
	RemovedManifests []v1.Hash
	Images           map[v1.Hash]v1.Image
}

// A Handler implementing ImageIndex.
// It will create and Manipulate IndexManifest.
//
// Prefer `IndexHandler` if underlying Images should need to be manipulated
type ManifestHandler struct {
	v1.ImageIndex
	Annotate         Annotate
	Options          IndexOptions
	RemovedManifests []v1.Hash
	Images           map[v1.Hash]v1.Descriptor
}

// An helper struct used for keeping track of changes made to ImageIndex.
type Annotate struct {
	Instance map[v1.Hash]v1.Descriptor
}

// Returns `OS` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) OS(hash v1.Hash) (os string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc, ok := a.Instance[hash]
	if !ok || desc.Platform == nil || desc.Platform.OS == "" {
		return os, ErrOSUndefined
	}

	return desc.Platform.OS, nil
}

// Sets the `OS` of an Image/ImageIndex to keep track of changes.
func (a *Annotate) SetOS(hash v1.Hash, os string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OS = os
	a.Instance[hash] = desc
}

// Returns `Architecture` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) Architecture(hash v1.Hash) (arch string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil || desc.Platform.Architecture == "" {
		return arch, ErrArchUndefined
	}

	return desc.Platform.Architecture, nil
}

// Annotates the `Architecture` of the given Image.
func (a *Annotate) SetArchitecture(hash v1.Hash, arch string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Architecture = arch
	a.Instance[hash] = desc
}

// Returns `Variant` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) Variant(hash v1.Hash) (variant string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil || desc.Platform.Variant == "" {
		return variant, ErrVariantUndefined
	}

	return desc.Platform.Variant, nil
}

// Annotates the `Variant` of the given Image.
func (a *Annotate) SetVariant(hash v1.Hash, variant string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Variant = variant
	a.Instance[hash] = desc
}

// Returns `OSVersion` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) OSVersion(hash v1.Hash) (osVersion string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil || desc.Platform.OSVersion == "" {
		return osVersion, ErrOSVersionUndefined
	}

	return desc.Platform.OSVersion, nil
}

// Annotates the `OSVersion` of the given Image.
func (a *Annotate) SetOSVersion(hash v1.Hash, osVersion string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OSVersion = osVersion
	a.Instance[hash] = desc
}

// Returns `Features` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) Features(hash v1.Hash) (features []string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil || len(desc.Platform.Features) == 0 {
		return features, ErrFeaturesUndefined
	}

	return desc.Platform.Features, nil
}

// Annotates the `Features` of the given Image.
func (a *Annotate) SetFeatures(hash v1.Hash, features []string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Features = features
	a.Instance[hash] = desc
}

// Returns `OSFeatures` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) OSFeatures(hash v1.Hash) (osFeatures []string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil || len(desc.Platform.OSFeatures) == 0 {
		return osFeatures, ErrOSFeaturesUndefined
	}

	return desc.Platform.OSFeatures, nil
}

// Annotates the `OSFeatures` of the given Image.
func (a *Annotate) SetOSFeatures(hash v1.Hash, osFeatures []string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OSFeatures = osFeatures
	a.Instance[hash] = desc
}

// Returns `Annotations` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) Annotations(hash v1.Hash) (annotations map[string]string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if len(desc.Annotations) == 0 {
		return annotations, ErrAnnotationsUndefined
	}

	return desc.Annotations, nil
}

// Annotates the `Annotations` of the given Image.
func (a *Annotate) SetAnnotations(hash v1.Hash, annotations map[string]string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Annotations = annotations
	a.Instance[hash] = desc
}

// Returns `URLs` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) URLs(hash v1.Hash) (urls []string, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if len(desc.URLs) == 0 {
		return urls, ErrURLsUndefined
	}

	return desc.URLs, nil
}

// Annotates the `URLs` of the given Image.
func (a *Annotate) SetURLs(hash v1.Hash, urls []string) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.URLs = urls
	a.Instance[hash] = desc
}

// Returns `types.MediaType` of an existing manipulated ImageIndex if found, else an error.
func (a *Annotate) Format(hash v1.Hash) (format types.MediaType, err error) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.MediaType == types.MediaType("") {
		return format, ErrUnknownMediaType
	}

	return desc.MediaType, nil
}

// Stores the `Format` of the given Image.
func (a *Annotate) SetFormat(hash v1.Hash, format types.MediaType) {
	if len(a.Instance) == 0 {
		a.Instance = make(map[v1.Hash]v1.Descriptor)
	}

	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.MediaType = format
	a.Instance[hash] = desc
}

// Returns `OS` of an existing Image.
func (h *ManifestHandler) OS(digest name.Digest) (os string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	// if any image is removed with given hash return an error
	for _, h := range h.RemovedManifests {
		if h == hash {
			return os, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	// if image is manipulated before return last manipulated value
	if os, err = h.Annotate.OS(hash); err == nil {
		return
	}

	// return the OS of the added image(using ImageIndex#Add) if found
	if desc, ok := h.Images[hash]; ok {
		if desc.Platform == nil {
			return os, ErrPlatformUndefined
		}

		if desc.Platform.OS == "" {
			return os, ErrOSUndefined
		}

		return desc.Platform.OS, nil
	}

	// check for the digest in the IndexManifest and return `OS` if found
	mfest, err := h.IndexManifest()
	if err != nil {
		return os, err
	}

	if mfest == nil {
		return os, ErrManifestUndefined
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if desc.Platform == nil {
				return os, ErrPlatformUndefined
			}

			if desc.Platform.OS == "" {
				return os, ErrOSUndefined
			}

			return desc.Platform.OS, nil
		}
	}

	// when no image found with the given digest return an error
	return os, ErrNoImageOrIndexFoundWithGivenDigest
}

// Returns `OS` of an existing Image.
func (i *IndexHandler) OS(digest name.Digest) (os string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	// if any image is removed with given hash return an error
	for _, h := range i.RemovedManifests {
		if h == hash {
			return os, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	// if image is manipulated before return last manipulated value
	if os, err = i.Annotate.OS(hash); err == nil {
		return
	}

	// return the OS of the added image(using ImageIndex#Add) if found
	if img, ok := i.Images[hash]; ok {
		return imageOS(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageOS(img)
}

func imageOS(img v1.Image) (os string, err error) {
	config, err := getConfigFile(img)
	if err != nil {
		return os, err
	}

	if config.OS == "" {
		return os, ErrOSUndefined
	}

	return config.OS, nil
}

// Annotates existing Image by updating `OS` field in IndexManifest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetOS(digest name.Digest, os string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	// return an error if the image is removed
	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	// if any nested imageIndex found with given digest save underlying image instead of index with the given OS
	if mfest, err := getIndexManifest(h, digest); err == nil {
		// keep track of changes until ImageIndex#Save is called
		h.Annotate.SetOS(hash, os)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	// set the `OS` of an Image from base ImageIndex if found
	if img, err := h.Image(hash); err == nil {
		return imageSetOS(h, img, hash, os)
	}

	// set the `OS` of an Image added to ImageIndex if found
	if desc, ok := h.Images[hash]; ok {
		// keep track of changes until ImageIndex#Save is called
		h.Annotate.SetOS(hash, os)
		h.Annotate.SetFormat(hash, desc.MediaType)

		return nil
	}

	// return an error if no Image found given digest
	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotate ImageIndex to update `OS` along with underlying Image.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) SetOS(digest name.Digest, os string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	// return an error if the Image is Removed
	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	// grab an Image from the Index with the given `OS` and update ImageIndex
	if mfest, err := getIndexManifest(i, digest); err == nil {
		i.Annotate.SetOS(hash, os)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	// set the `OS` of the image if found on base ImageIndex
	if img, err := i.Image(hash); err == nil {
		return imageSetOS(i, img, hash, os)
	}

	// set the `OS` for the image if found on newly added images. see IndexHandler#Save
	if img, ok := i.Images[hash]; ok {
		return imageSetOS(i, img, hash, os)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetOS(i ImageIndex, img v1.Image, hash v1.Hash, os string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetOS(hash, os)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetOS(hash, os)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}

	return nil
}

// Return the Architecture of an Image/Index based on given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) Architecture(digest name.Digest) (arch string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return arch, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if arch, err = h.Annotate.Architecture(hash); err == nil {
		return
	}

	if desc, ok := h.Images[hash]; ok {
		if desc.Platform == nil {
			return arch, ErrPlatformUndefined
		}

		if desc.Platform.Architecture == "" {
			return arch, ErrArchUndefined
		}

		return desc.Platform.Architecture, nil
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return arch, err
	}

	if mfest == nil {
		return arch, ErrManifestUndefined
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if desc.Platform == nil {
				return arch, ErrPlatformUndefined
			}

			if desc.Platform.Architecture == "" {
				return arch, ErrArchUndefined
			}

			return desc.Platform.Architecture, nil
		}
	}

	return arch, ErrNoImageOrIndexFoundWithGivenDigest
}

// Return the `Architecture` of an Image.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) Architecture(digest name.Digest) (arch string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return arch, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if arch, err = i.Annotate.Architecture(hash); err == nil {
		return
	}

	if img, ok := i.Images[hash]; ok {
		return imageArch(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageArch(img)
}

func imageArch(img v1.Image) (arch string, err error) {
	config, err := getConfigFile(img)
	if err != nil {
		return arch, err
	}

	if config.Architecture == "" {
		return arch, ErrArchUndefined
	}

	return config.Architecture, nil
}

// Annotates the `Architecture` of an Image.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetArchitecture(digest name.Digest, arch string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(h, digest); err == nil {
		h.Annotate.SetArchitecture(hash, arch)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return imageSetArch(h, img, hash, arch)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetArchitecture(hash, arch)
		h.Annotate.SetFormat(hash, desc.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `Architecture` of an Image
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) SetArchitecture(digest name.Digest, arch string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(i, digest); err == nil {
		i.Annotate.SetArchitecture(hash, arch)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetArch(i, img, hash, arch)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetArch(i, img, hash, arch)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetArch(i ImageIndex, img v1.Image, hash v1.Hash, arch string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetArchitecture(hash, arch)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetArchitecture(hash, arch)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}

	return nil
}

// Return the `Variant` of an Image.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) Variant(digest name.Digest) (osVariant string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return osVariant, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if osVariant, err = h.Annotate.Variant(hash); err == nil {
		return
	}

	if desc, ok := h.Images[hash]; ok {
		if desc.Platform == nil {
			return osVariant, ErrPlatformUndefined
		}

		if desc.Platform.Variant == "" {
			return osVariant, ErrVariantUndefined
		}

		return desc.Platform.Variant, nil
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return osVariant, err
	}

	if mfest == nil {
		return osVariant, ErrManifestUndefined
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if desc.Platform == nil {
				return osVariant, ErrPlatformUndefined
			}

			if desc.Platform.Variant == "" {
				return osVariant, ErrVariantUndefined
			}

			return desc.Platform.Variant, nil
		}
	}

	return osVariant, ErrNoImageOrIndexFoundWithGivenDigest
}

// Return the `Variant` of an Image with gievn Digest.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) Variant(digest name.Digest) (osVariant string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return osVariant, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if osVariant, err = i.Annotate.Variant(hash); err == nil {
		return
	}

	if img, ok := i.Images[hash]; ok {
		return imageVariant(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageVariant(img)
}

func imageVariant(img v1.Image) (osVariant string, err error) {
	config, err := getConfigFile(img)
	if err != nil {
		return osVariant, err
	}

	if config.Variant == "" {
		return osVariant, ErrVariantUndefined
	}

	return config.Variant, nil
}

// Annotates the `Variant` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetVariant(digest name.Digest, osVariant string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(h, digest); err == nil {
		h.Annotate.SetVariant(hash, osVariant)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return imageSetVariant(h, img, hash, osVariant)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetVariant(hash, osVariant)
		h.Annotate.SetFormat(hash, desc.MediaType)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `Variant` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) SetVariant(digest name.Digest, osVariant string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(i, digest); err == nil {
		i.Annotate.SetVariant(hash, osVariant)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetVariant(i, img, hash, osVariant)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetVariant(i, img, hash, osVariant)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetVariant(i ImageIndex, img v1.Image, hash v1.Hash, osVariant string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetVariant(hash, osVariant)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetVariant(hash, osVariant)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}

	return nil
}

// Returns the `OSVersion` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) OSVersion(digest name.Digest) (osVersion string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return osVersion, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if osVersion, err = h.Annotate.OSVersion(hash); err == nil {
		return
	}

	if desc, ok := h.Images[hash]; ok {
		if desc.Platform == nil {
			return osVersion, ErrPlatformUndefined
		}

		if desc.Platform.OSVersion == "" {
			return osVersion, ErrOSVersionUndefined
		}

		return desc.Platform.OSVersion, nil
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return osVersion, err
	}

	if mfest == nil {
		return osVersion, ErrManifestUndefined
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if desc.Platform == nil {
				return osVersion, ErrPlatformUndefined
			}

			if desc.Platform.OSVersion == "" {
				return osVersion, ErrOSVersionUndefined
			}

			return desc.Platform.OSVersion, nil
		}
	}

	return osVersion, ErrNoImageOrIndexFoundWithGivenDigest
}

// Returns the `OSVersion` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) OSVersion(digest name.Digest) (osVersion string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return osVersion, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if osVersion, err = i.Annotate.OSVersion(hash); err == nil {
		return
	}

	if img, ok := i.Images[hash]; ok {
		return imageOSVersion(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageOSVersion(img)
}

func imageOSVersion(img v1.Image) (osVersion string, err error) {
	config, err := getConfigFile(img)
	if err != nil {
		return osVersion, err
	}

	if config.OSVersion == "" {
		return osVersion, ErrOSVersionUndefined
	}

	return config.OSVersion, nil
}

// Annotates the `OSVersion` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetOSVersion(digest name.Digest, osVersion string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(h, digest); err == nil {
		h.Annotate.SetOSVersion(hash, osVersion)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return imageSetOSVersion(h, img, hash, osVersion)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetOSVersion(hash, osVersion)
		h.Annotate.SetFormat(hash, desc.MediaType)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `OSVersion` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) SetOSVersion(digest name.Digest, osVersion string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(i, digest); err == nil {
		i.Annotate.SetOSVersion(hash, osVersion)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetOSVersion(i, img, hash, osVersion)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetOSVersion(i, img, hash, osVersion)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetOSVersion(i ImageIndex, img v1.Image, hash v1.Hash, osVersion string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetOSVersion(hash, osVersion)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetOSVersion(hash, osVersion)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}

	return nil
}

// Returns the `Features` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) Features(digest name.Digest) (features []string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return features, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if features, err = h.Annotate.Features(hash); err == nil {
		return
	}

	features, err = indexFeatures(h, digest)
	if err == nil {
		return
	}

	if desc, ok := h.Images[hash]; ok {
		if desc.Platform == nil {
			return features, ErrPlatformUndefined
		}

		if len(desc.Platform.Features) == 0 {
			return features, ErrFeaturesUndefined
		}

		return desc.Platform.Features, nil
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return features, err
	}

	if mfest == nil {
		return features, ErrManifestUndefined
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if desc.Platform == nil {
				return features, ErrPlatformUndefined
			}

			if len(desc.Platform.Features) == 0 {
				return features, ErrFeaturesUndefined
			}

			return desc.Platform.Features, nil
		}
	}

	return features, ErrNoImageOrIndexFoundWithGivenDigest
}

// Returns the `Features` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) Features(digest name.Digest) (features []string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return features, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if features, err = i.Annotate.Features(hash); err == nil {
		return
	}

	features, err = indexFeatures(i, digest)
	if err == nil {
		return
	}

	if img, ok := i.Images[hash]; ok {
		return imageFeatures(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageFeatures(img)
}

func indexFeatures(i ImageIndex, digest name.Digest) (features []string, err error) {
	mfest, err := getIndexManifest(i, digest)
	if err != nil {
		return
	}

	if mfest.Subject == nil {
		mfest.Subject = &v1.Descriptor{}
	}

	if mfest.Subject.Platform == nil {
		mfest.Subject.Platform = &v1.Platform{}
	}

	if len(mfest.Subject.Platform.Features) == 0 {
		return features, ErrFeaturesUndefined
	}

	return mfest.Subject.Platform.Features, nil
}

func imageFeatures(img v1.Image) (features []string, err error) {
	config, err := getConfigFile(img)
	if err != nil {
		return features, err
	}

	platform := config.Platform()
	if platform == nil {
		return features, ErrConfigFilePlatformUndefined
	}

	if len(platform.Features) == 0 {
		return features, ErrFeaturesUndefined
	}

	return platform.Features, nil
}

// Annotates the `Features` of an Image with given Digest by appending to existsing Features if any.
//
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetFeatures(digest name.Digest, features []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(h, digest); err == nil {
		h.Annotate.SetFeatures(hash, features)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return imageSetFeatures(h, img, hash, features)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetFeatures(hash, features)
		h.Annotate.SetFormat(hash, desc.MediaType)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `Features` of an Image with given Digest by appending to existsing Features if any.
//
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) SetFeatures(digest name.Digest, features []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(i, digest); err == nil {
		i.Annotate.SetFeatures(hash, features)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetFeatures(i, img, hash, features)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetFeatures(i, img, hash, features)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetFeatures(i ImageIndex, img v1.Image, hash v1.Hash, features []string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetFeatures(hash, features)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetFeatures(hash, features)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}

	return nil
}

// Returns the `OSFeatures` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return osFeatures, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if osFeatures, err = h.Annotate.OSFeatures(hash); err == nil {
		return
	}

	osFeatures, err = indexOSFeatures(h, digest)
	if err == nil {
		return
	}

	if desc, ok := h.Images[hash]; ok {
		if desc.Platform == nil {
			return osFeatures, ErrPlatformUndefined
		}

		if len(desc.Platform.OSFeatures) == 0 {
			return osFeatures, ErrOSFeaturesUndefined
		}

		return desc.Platform.OSFeatures, nil
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return osFeatures, err
	}

	if mfest == nil {
		return osFeatures, ErrManifestUndefined
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if desc.Platform == nil {
				return osFeatures, ErrPlatformUndefined
			}

			if len(desc.Platform.OSFeatures) == 0 {
				return osFeatures, ErrOSFeaturesUndefined
			}

			return desc.Platform.OSFeatures, nil
		}
	}

	return osFeatures, ErrNoImageOrIndexFoundWithGivenDigest
}

// Returns the `OSFeatures` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return osFeatures, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if osFeatures, err = i.Annotate.OSFeatures(hash); err == nil {
		return
	}

	osFeatures, err = indexOSFeatures(i, digest)
	if err == nil {
		return
	}

	if img, ok := i.Images[hash]; ok {
		return imageOSFeatures(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageOSFeatures(img)
}

func indexOSFeatures(i ImageIndex, digest name.Digest) (osFeatures []string, err error) {
	mfest, err := getIndexManifest(i, digest)
	if err != nil {
		return
	}

	if mfest.Subject == nil {
		mfest.Subject = &v1.Descriptor{}
	}

	if mfest.Subject.Platform == nil {
		mfest.Subject.Platform = &v1.Platform{}
	}

	if len(mfest.Subject.Platform.OSFeatures) == 0 {
		return osFeatures, ErrOSFeaturesUndefined
	}

	return mfest.Subject.Platform.OSFeatures, nil
}

func imageOSFeatures(img v1.Image) (osFeatures []string, err error) {
	config, err := getConfigFile(img)
	if err != nil {
		return osFeatures, err
	}

	if len(config.OSFeatures) == 0 {
		return osFeatures, ErrOSFeaturesUndefined
	}

	return config.OSFeatures, nil
}

// Annotates the `OSFeatures` of an Image with given Digest by appending to existsing OSFeatures if any.
//
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(h, digest); err == nil {
		h.Annotate.SetOSFeatures(hash, osFeatures)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return imageSetOSFeatures(h, img, hash, osFeatures)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetOSFeatures(hash, osFeatures)
		h.Annotate.SetFormat(hash, desc.MediaType)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `OSFeatures` of an Image with given Digest by appending to existsing OSFeatures if any.
//
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(i, digest); err == nil {
		i.Annotate.SetOSFeatures(hash, osFeatures)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetOSFeatures(i, img, hash, osFeatures)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetOSFeatures(i, img, hash, osFeatures)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetOSFeatures(i ImageIndex, img v1.Image, hash v1.Hash, osFeatures []string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetOSFeatures(hash, osFeatures)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetOSFeatures(hash, osFeatures)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}

	return nil
}

// Return the `Annotations` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
//
// For Docker Images and Indexes it returns an error.
func (h *ManifestHandler) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return annotations, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if annotations, err = h.Annotate.Annotations(hash); err == nil {
		format, err := h.Annotate.Format(hash)
		switch format {
		case types.DockerManifestSchema2,
			types.DockerManifestSchema1,
			types.DockerManifestSchema1Signed,
			types.DockerManifestList:
			return nil, ErrAnnotationsUndefined
		case types.OCIManifestSchema1,
			types.OCIImageIndex:
			return annotations, err
		default:
			return annotations, ErrUnknownMediaType
		}
	}

	annotations, err = indexAnnotations(h, digest)
	if err == nil || errors.Is(err, ErrAnnotationsUndefined) {
		return annotations, err
	}

	if desc, ok := h.Images[hash]; ok {
		if len(desc.Annotations) == 0 {
			return annotations, ErrAnnotationsUndefined
		}

		return desc.Annotations, nil
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return annotations, err
	}

	if mfest == nil {
		return annotations, ErrManifestUndefined
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if len(desc.Annotations) == 0 {
				return annotations, ErrAnnotationsUndefined
			}

			return desc.Annotations, nil
		}
	}

	return annotations, ErrNoImageOrIndexFoundWithGivenDigest
}

// Return the `Annotations` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
//
// For Docker Images and Indexes it returns an error
func (i *IndexHandler) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return annotations, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if annotations, err = i.Annotate.Annotations(hash); err == nil {
		format, err := i.Annotate.Format(hash)
		switch format {
		case types.DockerManifestSchema2,
			types.DockerManifestSchema1,
			types.DockerManifestSchema1Signed,
			types.DockerManifestList:
			return nil, ErrAnnotationsUndefined
		case types.OCIManifestSchema1,
			types.OCIImageIndex:
			return annotations, err
		default:
			return annotations, ErrUnknownMediaType
		}
	}

	annotations, err = indexAnnotations(i, digest)
	if err == nil || errors.Is(err, ErrAnnotationsUndefined) {
		return annotations, err
	}

	if img, ok := i.Images[hash]; ok {
		return imageAnnotations(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageAnnotations(img)
}

func indexAnnotations(i ImageIndex, digest name.Digest) (annotations map[string]string, err error) {
	mfest, err := getIndexManifest(i, digest)
	if err != nil {
		return
	}

	if len(mfest.Annotations) == 0 {
		return annotations, ErrAnnotationsUndefined
	}

	if mfest.MediaType == types.DockerManifestList {
		return nil, ErrAnnotationsUndefined
	}

	return mfest.Annotations, nil
}

func imageAnnotations(img v1.Image) (annotations map[string]string, err error) {
	mfest, err := img.Manifest()
	if err != nil {
		return annotations, err
	}

	if mfest == nil {
		return annotations, ErrManifestUndefined
	}

	if len(mfest.Annotations) == 0 {
		return annotations, ErrAnnotationsUndefined
	}

	switch mfest.MediaType {
	case types.DockerManifestSchema2,
		types.DockerManifestSchema1,
		types.DockerManifestSchema1Signed,
		types.DockerManifestList:
		return nil, ErrAnnotationsUndefined
	case types.OCIImageIndex,
		types.OCIManifestSchema1:
		return mfest.Annotations, nil
	default:
		return nil, ErrUnknownMediaType
	}
}

// Annotates the `Annotations` of an Image with given Digest by appending to existsing Annotations if any.
//
// Returns an error if no Image/Index found with given Digest.
//
// For Docker Images and Indexes it ignores updating Annotations.
func (h *ManifestHandler) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if idx, err := h.ImageIndex.ImageIndex(hash); err == nil {
		mfest, err := idx.IndexManifest()
		if err != nil {
			return err
		}

		annos := mfest.Annotations
		if len(annos) == 0 {
			annos = make(map[string]string)
		}

		for k, v := range annotations {
			annos[k] = v
		}

		h.Annotate.SetAnnotations(hash, annos)
		h.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return imageSetAnnotations(h, img, hash, annotations)
	}

	if desc, ok := h.Images[hash]; ok {
		annos := make(map[string]string, 0)
		if len(desc.Annotations) != 0 {
			annos = desc.Annotations
		}

		for k, v := range annotations {
			annos[k] = v
		}

		h.Annotate.SetAnnotations(hash, annos)
		h.Annotate.SetFormat(hash, desc.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `Annotations` of an Image with given Digest by appending to existsing Annotations if any.
//
// Returns an error if no Image/Index found with given Digest.
//
// For Docker Images and Indexes ignore updating Annotations
func (i *IndexHandler) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if idx, err := i.ImageIndex.ImageIndex(hash); err == nil {
		mfest, err := idx.IndexManifest()
		if err != nil {
			return err
		}

		annos := mfest.Annotations
		if len(annos) == 0 {
			annos = make(map[string]string)
		}

		for k, v := range annotations {
			annos[k] = v
		}

		i.Annotate.SetAnnotations(hash, annos)
		i.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetAnnotations(i, img, hash, annotations)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetAnnotations(i, img, hash, annotations)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetAnnotations(i ImageIndex, img v1.Image, hash v1.Hash, annotations map[string]string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	annos := mfest.Annotations
	if len(annos) == 0 {
		annos = make(map[string]string)
	}

	for k, v := range annotations {
		annos[k] = v
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetAnnotations(hash, annos)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetAnnotations(hash, annos)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}
	return nil
}

// Returns the `URLs` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) URLs(digest name.Digest) (urls []string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return urls, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if urls, err = h.Annotate.URLs(hash); err == nil {
		return
	}

	urls, err = getIndexURLs(h, hash)
	if err == nil {
		return
	}

	urls, err = getImageURLs(h, hash)
	if err == nil {
		return
	}

	if err == ErrURLsUndefined {
		return urls, ErrURLsUndefined
	}

	return urls, ErrNoImageOrIndexFoundWithGivenDigest
}

// Returns the `URLs` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) URLs(digest name.Digest) (urls []string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return urls, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if urls, err = i.Annotate.URLs(hash); err == nil {
		return
	}

	urls, err = getIndexURLs(i, hash)
	if err == nil {
		return
	}

	urls, err = getImageURLs(i, hash)
	if err == nil {
		return
	}

	if err == ErrURLsUndefined {
		return urls, ErrURLsUndefined
	}

	return urls, ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `URLs` of an Image with given Digest by appending to existsing URLs if any.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetURLs(digest name.Digest, urls []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range h.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(h, digest); err == nil {
		h.Annotate.SetURLs(hash, urls)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return imageSetURLs(h, img, hash, urls)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetURLs(hash, urls)
		h.Annotate.SetFormat(hash, desc.MediaType)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

// Annotates the `URLs` of an Image with given Digest by appending to existsing URLs if any.
// Returns an error if no Image/Index found with given Digest.
func (i *IndexHandler) SetURLs(digest name.Digest, urls []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(i, digest); err == nil {
		i.Annotate.SetURLs(hash, urls)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetURLs(i, img, hash, urls)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetURLs(i, img, hash, urls)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetURLs(i ImageIndex, img v1.Image, hash v1.Hash, urls []string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	switch i := i.(type) {
	case *IndexHandler:
		i.Annotate.SetURLs(hash, urls)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	case *ManifestHandler:
		i.Annotate.SetURLs(hash, urls)
		i.Annotate.SetFormat(hash, mfest.MediaType)
	default:
		return ErrUnknownHandler
	}

	return nil
}

// Add the ImageIndex from the registry with the given Reference.
//
// If referencing an ImageIndex, will add Platform Specific Image from the Index.
// Use IndexAddOptions to alter behaviour for ImageIndex Reference.
func (h *ManifestHandler) Add(ref name.Reference, ops ...IndexAddOption) error {
	var addOps = &AddOptions{}
	for _, op := range ops {
		op(addOps)
	}

	// Fetch Descriptor of the given reference.
	//
	// This call is returns a v1.Descriptor with `Size`, `MediaType`, `Digest` fields only!!
	// This is a light weight call used for checking MediaType of given Reference
	desc, err := remote.Head(
		ref,
		remote.WithAuthFromKeychain(h.Options.KeyChain),
		remote.WithTransport(getTransport(h.Options.Insecure())),
	)
	if err != nil {
		return err
	}

	if desc == nil {
		return ErrManifestUndefined
	}

	switch {
	case desc.MediaType.IsImage():
		// Get the Full Image from remote if the given Reference refers an Image
		img, err := remote.Image(
			ref,
			remote.WithAuthFromKeychain(h.Options.KeyChain),
			remote.WithTransport(getTransport(h.Options.Insecure())),
		)
		if err != nil {
			return err
		}

		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		imgConfig, err := img.ConfigFile()
		if err != nil {
			return err
		}

		if imgConfig == nil {
			return ErrConfigFileUndefined
		}

		platform := v1.Platform{}
		updatePlatform(imgConfig, &platform)

		// update the v1.Descriptor with expected MediaType, Size, and Digest
		// since mfest.Subject can be nil using mfest.Config is safer
		config := mfest.Config
		config.Digest = desc.Digest
		config.MediaType = desc.MediaType
		config.Size = desc.Size
		config.Platform = &platform
		config.Annotations = mfest.Annotations

		// keep tract of newly added Image
		h.Images[desc.Digest] = config

		if config.MediaType == types.OCIManifestSchema1 && len(addOps.Annotations) != 0 {
			if len(config.Annotations) == 0 {
				config.Annotations = make(map[string]string)
			}

			for k, v := range addOps.Annotations {
				config.Annotations[k] = v
			}
		}

		layoutPath := filepath.Join(h.Options.XdgPath, h.Options.Reponame)
		path, err := layout.FromPath(layoutPath)
		if err != nil {
			path, err = layout.Write(layoutPath, h.ImageIndex)
			if err != nil {
				return err
			}
		}

		// Append Image to V1.ImageIndex with the Annotations if any
		return path.AppendDescriptor(config)
	case desc.MediaType.IsIndex():
		switch {
		case addOps.All:
			desc, err := remote.Get(
				ref,
				remote.WithAuthFromKeychain(h.Options.KeyChain),
				remote.WithTransport(getTransport(h.Options.Insecure())),
			)
			if err != nil {
				return err
			}

			idx, err := desc.ImageIndex()
			if err != nil {
				return err
			}

			var wg sync.WaitGroup
			var iMap sync.Map
			errs := SaveError{}
			// Add all the Images from Nested ImageIndexes
			err = addAllImages(h, &idx, addOps.Annotations, &wg, &iMap)
			if err != nil {
				return err
			}

			wg.Wait()
			layoutPath := filepath.Join(h.Options.XdgPath, h.Options.Reponame)
			path, err := layout.FromPath(layoutPath)
			if err != nil {
				// if the ImageIndex is not saved till now for some reason Save the ImageIndex locally to append Images
				err = h.Save()
				if err != nil {
					return err
				}
			}

			iMap.Range(func(key, value any) bool {
				desc, ok := value.(v1.Descriptor)
				if !ok {
					return false
				}

				digest, ok := key.(v1.Hash)
				if !ok {
					return false
				}

				h.Images[digest] = desc

				// Append All the Images within the nested ImageIndexes
				err = path.AppendDescriptor(desc)
				if err != nil {
					errs.Errors = append(errs.Errors, SaveDiagnostic{
						Cause: err,
					})
				}
				return true
			})

			if len(errs.Errors) != 0 {
				return errs
			}

			return nil
		case addOps.OS != "",
			addOps.Arch != "",
			addOps.Variant != "",
			addOps.OSVersion != "",
			len(addOps.Features) != 0,
			len(addOps.OSFeatures) != 0:
			platformSpecificDesc := &v1.Platform{}
			if addOps.OS != "" {
				platformSpecificDesc.OS = addOps.OS
			}

			if addOps.Arch != "" {
				platformSpecificDesc.Architecture = addOps.Arch
			}

			if addOps.Variant != "" {
				platformSpecificDesc.Variant = addOps.Variant
			}

			if addOps.OSVersion != "" {
				platformSpecificDesc.OSVersion = addOps.OSVersion
			}

			if len(addOps.Features) != 0 {
				platformSpecificDesc.Features = addOps.Features
			}

			if len(addOps.OSFeatures) != 0 {
				platformSpecificDesc.OSFeatures = addOps.OSFeatures
			}

			// Add an Image from the ImageIndex with the given Platform
			return addPlatformSpecificImages(h, ref, *platformSpecificDesc, addOps.Annotations)
		default:
			platform := v1.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
			}

			// Add the Image from the ImageIndex with current Device's Platform
			return addPlatformSpecificImages(h, ref, platform, addOps.Annotations)
		}
	default:
		// return an error if the Reference is neither an Image not an Index
		return ErrUnknownMediaType
	}
}

// Add the ImageIndex from the registry with the given Reference.
//
// If referencing an ImageIndex, will add Platform Specific Image from the Index.
// Use IndexAddOptions to alter behaviour for ImageIndex Reference.
//
// It will locally store all the blobs of the added Images.
func (i *IndexHandler) Add(ref name.Reference, ops ...IndexAddOption) error {
	var addOps = &AddOptions{}
	for _, op := range ops {
		op(addOps)
	}

	// Fetch Descriptor of the given reference.
	//
	// This call is returns a v1.Descriptor with `Size`, `MediaType`, `Digest` fields only!!
	// This is a light weight call used for checking MediaType of the Reference
	desc, err := remote.Head(
		ref,
		remote.WithAuthFromKeychain(i.Options.KeyChain),
		remote.WithTransport(getTransport(i.Options.Insecure())),
	)
	if err != nil {
		return err
	}

	if desc == nil {
		return ErrManifestUndefined
	}

	switch {
	case desc.MediaType.IsImage():
		// Get the Full Image from remote if the given Reference refers an Image
		img, err := remote.Image(
			ref,
			remote.WithAuthFromKeychain(i.Options.KeyChain),
			remote.WithTransport(getTransport(i.Options.Insecure())),
		)
		if err != nil {
			return err
		}

		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		// use layout options to update IndexManifest with the given Annotations along with Platform and URLs
		var layoutOps []layout.Option
		annos := mfest.Annotations
		if desc.MediaType == types.OCIManifestSchema1 && len(addOps.Annotations) != 0 {
			if len(annos) == 0 {
				annos = make(map[string]string)
			}

			for k, v := range addOps.Annotations {
				annos[k] = v
			}

			layoutOps = append(layoutOps, layout.WithAnnotations(annos))
			// Mutate the Image with the Given Annotations.
			// Note: It will only updates the Annotations at mfest.Config.Annotations
			img = mutate.Annotations(img, annos).(v1.Image)
			i.Annotate.SetAnnotations(desc.Digest, annos)
			i.Annotate.SetFormat(desc.Digest, desc.MediaType)
		}

		if len(mfest.Config.URLs) != 0 {
			layoutOps = append(layoutOps, layout.WithURLs(mfest.Config.URLs))
		}

		var platform *v1.Platform
		if platform = mfest.Config.Platform; platform == nil || platform.Equals(v1.Platform{}) {
			if platform == nil {
				platform = &v1.Platform{}
			}

			config, err := img.ConfigFile()
			if err != nil {
				return err
			}

			if config == nil {
				return ErrConfigFileUndefined
			}

			if err = updatePlatform(config, platform); err != nil {
				return err
			}

			layoutOps = append(layoutOps, layout.WithPlatform(*platform))
		}

		layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
		path, err := layout.FromPath(layoutPath)
		if err != nil {
			path, err = layout.Write(layoutPath, i.ImageIndex)
			if err != nil {
				return err
			}
		}

		// keep track of newly added Image
		i.Images[desc.Digest] = img
		// write Image along with its Blobs locally and update IndexManifest
		return path.AppendImage(img, layoutOps...)
	case desc.MediaType.IsIndex():
		switch {
		case addOps.All:
			idx, err := remote.Index(
				ref,
				remote.WithAuthFromKeychain(i.Options.KeyChain),
				remote.WithTransport(getTransport(i.Options.Insecure())),
			)
			if err != nil {
				return err
			}

			var wg sync.WaitGroup
			var imageMap sync.Map
			errs := SaveError{}

			// Append All the Images within the nested ImageIndex by pushing all images to `imageMap`
			err = addAllImages(i, &idx, addOps.Annotations, &wg, &imageMap)
			if err != nil {
				return err
			}

			wg.Wait()
			layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
			path, err := layout.FromPath(layoutPath)
			if err != nil {
				err = i.Save()
				if err != nil {
					return err
				}
			}

			imageMap.Range(func(key, value any) bool {
				img, ok := key.(v1.Image)
				if !ok {
					return false
				}

				ops, ok := value.([]layout.Option)
				if !ok {
					return false
				}

				// Append Image with the given `layout.Option`s
				err = path.AppendImage(img, ops...)
				if err != nil {
					errs.Errors = append(errs.Errors, SaveDiagnostic{
						Cause: err,
					})
				}
				return true
			})

			if len(errs.Errors) != 0 {
				return errs
			}

			return nil
		case addOps.OS != "",
			addOps.Arch != "",
			addOps.Variant != "",
			addOps.OSVersion != "",
			len(addOps.Features) != 0,
			len(addOps.OSFeatures) != 0:
			platformSpecificDesc := &v1.Platform{}
			if addOps.OS != "" {
				platformSpecificDesc.OS = addOps.OS
			}

			if addOps.Arch != "" {
				platformSpecificDesc.Architecture = addOps.Arch
			}

			if addOps.Variant != "" {
				platformSpecificDesc.Variant = addOps.Variant
			}

			if addOps.OSVersion != "" {
				platformSpecificDesc.OSVersion = addOps.OSVersion
			}

			if len(addOps.Features) != 0 {
				platformSpecificDesc.Features = addOps.Features
			}

			if len(addOps.OSFeatures) != 0 {
				platformSpecificDesc.OSFeatures = addOps.OSFeatures
			}

			// add Image from the Given Index with the given Platform
			return addPlatformSpecificImages(i, ref, *platformSpecificDesc, addOps.Annotations)
		default:
			platform := v1.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
			}

			// Add Image with the target's specific device platform
			return addPlatformSpecificImages(i, ref, platform, addOps.Annotations)
		}
	default:
		// return an error if the Reference is neither an Image not an Index
		return ErrNoImageOrIndexFoundWithGivenDigest
	}
}

func updatePlatform(config *v1.ConfigFile, platform *v1.Platform) error {
	if config == nil {
		return ErrConfigFileUndefined
	}

	if platform == nil {
		return ErrPlatformUndefined
	}

	if platform.OS == "" {
		platform.OS = config.OS
	}

	if platform.Architecture == "" {
		platform.Architecture = config.Architecture
	}

	if platform.Variant == "" {
		platform.Variant = config.Variant
	}

	if platform.OSVersion == "" {
		platform.OSVersion = config.OSVersion
	}

	if len(platform.Features) == 0 {
		p := config.Platform()
		if p == nil {
			p = &v1.Platform{}
		}

		platform.Features = p.Features
	}

	if len(platform.OSFeatures) == 0 {
		platform.OSFeatures = config.OSFeatures
	}

	return nil
}

func addAllImages(i ImageIndex, idx *v1.ImageIndex, annotations map[string]string, wg *sync.WaitGroup, imageMap *sync.Map) error {
	mfest, err := (*idx).IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	errs := SaveError{}
	for _, desc := range mfest.Manifests {
		wg.Add(1)
		go func(desc v1.Descriptor) {
			defer wg.Done()
			err = addIndexAddendum(i, annotations, desc, idx, wg, imageMap)
			if err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					ImageName: desc.Digest.String(),
					Cause:     err,
				})
			}
		}(desc)
	}

	if len(errs.Errors) != 0 {
		return errs
	}

	return nil
}

func addIndexAddendum(i ImageIndex, annotations map[string]string, desc v1.Descriptor, idx *v1.ImageIndex, wg *sync.WaitGroup, iMap *sync.Map) error {
	switch i := i.(type) {
	case *IndexHandler:
		switch {
		case desc.MediaType.IsIndex():
			ii, err := (*idx).ImageIndex(desc.Digest)
			if err != nil {
				return err
			}

			return addAllImages(i, &ii, annotations, wg, iMap)
		case desc.MediaType.IsImage():
			img, err := (*idx).Image(desc.Digest)
			if err != nil {
				return err
			}

			mfest, err := img.Manifest()
			if err != nil {
				return err
			}

			if mfest == nil {
				return ErrManifestUndefined
			}

			if mfest.Subject == nil {
				mfest.Subject = &v1.Descriptor{}
			}

			var annos = make(map[string]string)
			var ops []layout.Option
			if len(annotations) != 0 && mfest.MediaType == types.OCIManifestSchema1 {
				if len(mfest.Annotations) != 0 {
					annos = mfest.Annotations
				}

				for k, v := range annotations {
					annos[k] = v
				}

				ops = append(ops, layout.WithAnnotations(annos))
				img = mutate.Annotations(img, annos).(v1.Image)
			}

			if len(mfest.Config.URLs) != 0 {
				ops = append(ops, layout.WithURLs(mfest.Config.URLs))
			}

			if platform := mfest.Config.Platform; platform == nil || platform.Equals(v1.Platform{}) {
				if platform == nil {
					platform = &v1.Platform{}
				}

				config, err := img.ConfigFile()
				if err != nil {
					return err
				}

				if config == nil {
					return ErrConfigFileUndefined
				}

				if err = updatePlatform(config, platform); err != nil {
					return err
				}

				ops = append(ops, layout.WithPlatform(*platform))
			}

			i.Images[desc.Digest] = img
			iMap.Store(img, ops)
			return nil
		default:
			return ErrUnknownMediaType
		}
	case *ManifestHandler:
		switch {
		case desc.MediaType.IsIndex():
			ii, err := (*idx).ImageIndex(desc.Digest)
			if err != nil {
				return err
			}

			return addAllImages(i, &ii, annotations, wg, iMap)
		case desc.MediaType.IsImage():
			img, err := (*idx).Image(desc.Digest)
			if err != nil {
				return err
			}

			mfest, err := img.Manifest()
			if err != nil {
				return err
			}

			if mfest == nil {
				return ErrManifestUndefined
			}

			imgConfig, err := img.ConfigFile()
			if err != nil {
				return err
			}

			if imgConfig == nil {
				return ErrConfigFileUndefined
			}

			platform := v1.Platform{}
			err = updatePlatform(imgConfig, &platform)
			if err != nil {
				return err
			}

			config := mfest.Config.DeepCopy()
			config.Size = desc.Size
			config.MediaType = desc.MediaType
			config.Digest = desc.Digest
			config.Platform = &platform
			config.Annotations = mfest.Annotations

			if len(config.Annotations) == 0 {
				config.Annotations = make(map[string]string, 0)
			}

			if len(annotations) != 0 && mfest.MediaType == types.OCIManifestSchema1 {
				for k, v := range annotations {
					config.Annotations[k] = v
				}
			}

			i.Images[desc.Digest] = *config
			iMap.Store(desc.Digest, *config)

			return nil
		default:
			return ErrUnknownMediaType
		}
	default:
		return ErrUnknownHandler
	}
}

func addPlatformSpecificImages(i ImageIndex, ref name.Reference, platform v1.Platform, annotations map[string]string) error {
	if platform.OS == "" || platform.Architecture == "" {
		return ErrInvalidPlatform
	}

	switch i := i.(type) {
	case *IndexHandler:
		desc, err := remote.Get(
			ref,
			remote.WithAuthFromKeychain(i.Options.KeyChain),
			remote.WithTransport(getTransport(true)),
			remote.WithPlatform(platform),
		)
		if err != nil {
			return err
		}

		return appendImage(i, desc, annotations)
	case *ManifestHandler:
		desc, err := remote.Get(
			ref,
			remote.WithAuthFromKeychain(i.Options.KeyChain),
			remote.WithTransport(getTransport(true)),
			remote.WithPlatform(platform),
		)
		if err != nil {
			return err
		}

		img, err := desc.Image()
		if err != nil {
			return err
		}

		digest, err := img.Digest()
		if err != nil {
			return err
		}

		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		imgConfig, err := img.ConfigFile()
		if err != nil {
			return err
		}

		if imgConfig == nil {
			return ErrConfigFileUndefined
		}

		platform := v1.Platform{}
		if err = updatePlatform(imgConfig, &platform); err != nil {
			return err
		}

		config := mfest.Config.DeepCopy()
		config.MediaType = mfest.MediaType
		config.Digest = digest
		config.Size = desc.Size
		config.Platform = &platform
		config.Annotations = mfest.Annotations

		if len(config.Annotations) != 0 {
			config.Annotations = make(map[string]string, 0)
		}

		if len(annotations) != 0 && config.MediaType == types.OCIManifestSchema1 {
			for k, v := range annotations {
				config.Annotations[k] = v
			}
		}

		i.Images[digest] = *config

		layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
		path, err := layout.FromPath(layoutPath)
		if err != nil {
			path, err = layout.Write(layoutPath, i.ImageIndex)
			if err != nil {
				return err
			}
		}

		return path.AppendDescriptor(*config)
	default:
		return ErrUnknownHandler
	}
}

func appendImage(i *IndexHandler, desc *remote.Descriptor, annotations map[string]string) error {
	img, err := desc.Image()
	if err != nil {
		return err
	}

	digest, err := img.Digest()
	if err != nil {
		return err
	}

	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	var layoutOps []layout.Option
	var annos = make(map[string]string)
	if len(annotations) != 0 && mfest.MediaType == types.OCIManifestSchema1 {
		if len(mfest.Annotations) != 0 {
			annos = mfest.Annotations
		}

		for k, v := range annotations {
			annos[k] = v
		}

		layoutOps = append(layoutOps, layout.WithAnnotations(annos))
		// i.Annotate.SetAnnotations(digest, annos)
		// i.Annotate.SetFormat(digest, desc.MediaType)
		img = mutate.Annotations(img, annos).(v1.Image)
	}

	if len(mfest.Config.URLs) != 0 {
		layoutOps = append(layoutOps, layout.WithURLs(mfest.Config.URLs))
	}

	if platform := mfest.Config.Platform; platform == nil || platform.Equals(v1.Platform{}) {
		if platform == nil {
			platform = &v1.Platform{}
		}

		config, err := img.ConfigFile()
		if err != nil {
			return err
		}

		if config == nil {
			return ErrConfigFileUndefined
		}

		if err = updatePlatform(config, platform); err != nil {
			return err
		}

		layoutOps = append(layoutOps, layout.WithPlatform(*platform))
	}

	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		path, err = layout.Write(layoutPath, i.ImageIndex)
		if err != nil {
			return err
		}
	}

	i.Images[digest] = img
	return path.AppendImage(img, layoutOps...)
}

// Save will locally save the given ImageIndex.
func (h *ManifestHandler) Save() error {
	layoutPath := filepath.Join(h.Options.XdgPath, h.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		// If the ImageIndex is not been saved before Save the ImageIndex
		mfest, err := h.IndexManifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		// Initially write an empty IndexManifest with expected MediaType
		if mfest.MediaType == types.OCIImageIndex {
			path, err = layout.Write(layoutPath, empty.Index)
			if err != nil {
				return err
			}
		} else {
			path, err = layout.Write(layoutPath, docker.Index)
			if err != nil {
				return err
			}
		}

		// loop over each digest and append assositated Image/ImageIndex
		for _, d := range mfest.Manifests {
			switch {
			case d.MediaType.IsIndex(), d.MediaType.IsImage():
				if err = path.AppendDescriptor(d); err != nil {
					return err
				}
			default:
				return ErrUnknownMediaType
			}
		}
	}

	hashes := make([]v1.Hash, 0, len(h.Annotate.Instance))
	for h := range h.Annotate.Instance {
		hashes = append(hashes, h)
	}

	// Remove all the Annotated Images/ImageIndexes from local ImageIndex to avoid duplicate Images with same Digest
	err = path.RemoveDescriptors(match.Digests(hashes...))
	if err != nil {
		return err
	}

	var errs SaveError
	for hash, desc := range h.Annotate.Instance {
		// If the digest matches an Image added annotate the Image and Save Locally
		if imgDesc, ok := h.Images[hash]; ok {
			if len(desc.Annotations) != 0 {
				if len(imgDesc.Annotations) == 0 {
					imgDesc.Annotations = make(map[string]string, 0)
				}

				for k, v := range desc.Annotations {
					imgDesc.Annotations[k] = v
				}
			}

			if len(desc.URLs) != 0 {
				imgDesc.URLs = append(imgDesc.URLs, desc.URLs...)
			}

			if p := desc.Platform; p != nil {
				if imgDesc.Platform == nil {
					imgDesc.Platform = &v1.Platform{}
				}

				if p.OS != "" {
					imgDesc.Platform.OS = p.OS
				}

				if p.Architecture != "" {
					imgDesc.Platform.Architecture = p.Architecture
				}

				if p.Variant != "" {
					imgDesc.Platform.Variant = p.Variant
				}

				if p.OSVersion != "" {
					imgDesc.Platform.OSVersion = p.OSVersion
				}

				if len(p.Features) != 0 {
					imgDesc.Platform.Features = append(imgDesc.Platform.Features, p.Features...)
				}

				if len(p.OSFeatures) != 0 {
					imgDesc.Platform.OSFeatures = append(imgDesc.Platform.OSFeatures, p.OSFeatures...)
				}
			}

			path.RemoveDescriptors(match.Digests(imgDesc.Digest))
			if err := path.AppendDescriptor(imgDesc); err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					Cause: err,
				})
			}

			continue
		}

		// If an Image with the given Digest exists annotate and Save it locally
		img, err := h.Image(hash)
		if err != nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				Cause: err,
			})
			continue
		}

		mfest, err := img.Manifest()
		if err != nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				Cause: err,
			})
			continue
		}

		if mfest == nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				Cause: ErrManifestUndefined,
			})
			continue
		}

		config, err := img.ConfigFile()
		if err != nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				Cause: err,
			})
			continue
		}

		if config == nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				Cause: ErrConfigFileUndefined,
			})
			continue
		}

		mfestSubject := mfest.Config.DeepCopy()
		mfestSubject.Annotations = mfest.Annotations
		mfestSubject.Digest = hash
		mfestSubject.MediaType = mfest.MediaType

		if len(desc.Annotations) != 0 && (mfest.MediaType == types.OCIImageIndex || mfest.MediaType == types.OCIManifestSchema1) {
			if len(mfestSubject.Annotations) == 0 {
				mfestSubject.Annotations = make(map[string]string, 0)
			}

			for k, v := range desc.Annotations {
				mfestSubject.Annotations[k] = v
			}
		}

		if len(desc.URLs) != 0 {
			mfestSubject.URLs = append(mfestSubject.URLs, desc.URLs...)
		}

		platform := v1.Platform{}
		if err = updatePlatform(config, &platform); err != nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				Cause: ErrConfigFileUndefined,
			})
			continue
		}

		if p := desc.Platform; p != nil {
			if mfestSubject.Platform == nil {
				mfestSubject.Platform = &v1.Platform{}
			}

			if p.OS != "" {
				platform.OS = p.OS
			}

			if p.Architecture != "" {
				platform.Architecture = p.Architecture
			}

			if p.Variant != "" {
				platform.Variant = p.Variant
			}

			if p.OSVersion != "" {
				platform.OSVersion = p.OSVersion
			}

			if len(p.Features) != 0 {
				platform.Features = append(platform.Features, p.Features...)
			}

			if len(p.OSFeatures) != 0 {
				platform.OSFeatures = append(platform.OSFeatures, p.OSFeatures...)
			}
		}

		mfestSubject.Platform = &platform
		path.RemoveDescriptors(match.Digests(mfestSubject.Digest))
		if err := path.AppendDescriptor(*mfestSubject); err != nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				Cause: err,
			})
		}
	}

	if len(errs.Errors) != 0 {
		return errs
	}

	var removeHashes = make([]v1.Hash, 0)
	for _, hash := range h.RemovedManifests {
		if _, ok := h.Images[hash]; !ok {
			removeHashes = append(removeHashes, hash)
			delete(h.Images, hash)
		}
	}

	h.Annotate = Annotate{
		Instance: make(map[v1.Hash]v1.Descriptor, 0),
	}
	h.RemovedManifests = make([]v1.Hash, 0)
	return path.RemoveDescriptors(match.Digests(removeHashes...))
}

// Save the ImageIndex locally
func (i *IndexHandler) Save() error {
	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		// write an ImageIndex locally with all the Blobs
		path, err = layout.Write(layoutPath, i.ImageIndex)
		if err != nil {
			return err
		}
	}

	hashes := make([]v1.Hash, 0, len(i.Annotate.Instance))
	for h := range i.Annotate.Instance {
		hashes = append(hashes, h)
	}

	// remove all the manifests that needs to be annotated to avoid duplicate Digests
	err = path.RemoveDescriptors(match.Digests(hashes...))
	if err != nil {
		return err
	}

	var errs SaveError
	var wg sync.WaitGroup
	var iMap sync.Map
	errGroup, _ := errgroup.WithContext(context.Background())
	for hash, desc := range i.Annotate.Instance {
		switch {
		case desc.MediaType.IsIndex():
			wg.Add(1)
			errGroup.Go(func() error {
				defer wg.Done()

				ii, err := i.ImageIndex.ImageIndex(hash)
				if err != nil {
					return err
				}

				mfest, err := ii.IndexManifest()
				if err != nil {
					return err
				}

				if mfest == nil {
					return ErrManifestUndefined
				}

				var ops []layout.Option
				if len(desc.Annotations) != 0 && desc.MediaType == types.OCIImageIndex {
					var annos = make(map[string]string)
					if len(mfest.Annotations) != 0 {
						annos = mfest.Annotations
					}

					for k, v := range desc.Annotations {
						annos[k] = v
					}
					ops = append(ops, layout.WithAnnotations(annos))
					if mfest.Subject == nil {
						mfest.Subject = &v1.Descriptor{}
					}
					var upsertSubject = mfest.Subject.DeepCopy()
					upsertSubject.Annotations = annos
					ii = mutate.Subject(
						mutate.Annotations(ii, annos).(v1.ImageIndex),
						*upsertSubject,
					).(v1.ImageIndex)
				}

				iMap.Store(ii, ops)
				return nil
			})

			if err = errGroup.Wait(); err != nil {
				return err
			}
		case desc.MediaType.IsImage():
			if _, ok := i.Images[hash]; ok {
				continue
			}

			wg.Add(1)
			errGroup.Go(func() error {
				defer wg.Done()

				img, err := i.Image(hash)
				if err != nil {
					return err
				}

				config, err := img.ConfigFile()
				if err != nil {
					return err
				}

				if config == nil {
					return ErrConfigFileUndefined
				}

				mfest, err := img.Manifest()
				if err != nil {
					return err
				}

				if mfest == nil {
					return ErrManifestUndefined
				}

				var ops []layout.Option
				var upsertSubject = mfest.Config.DeepCopy()
				var upsertConfig = config.DeepCopy()
				if upsertSubject == nil {
					upsertSubject = &v1.Descriptor{}
				}

				upsertSubject.Digest = desc.Digest
				upsertSubject.Size = desc.Size
				upsertSubject.MediaType = desc.MediaType

				if upsertConfig == nil {
					upsertConfig = &v1.ConfigFile{}
				}

				if upsertSubject.Platform == nil {
					upsertSubject.Platform = &v1.Platform{}
				}

				err = updatePlatform(config, upsertSubject.Platform)
				if err != nil {
					return err
				}

				if platform := desc.Platform; platform != nil && !platform.Equals(v1.Platform{}) {
					if platform.OS != "" {
						upsertConfig.OS = platform.OS
						if upsertSubject.Platform == nil {
							upsertSubject.Platform = &v1.Platform{}
						}

						upsertSubject.Platform.OS = platform.OS
					}

					if platform.Architecture != "" {
						upsertConfig.Architecture = platform.Architecture
						if upsertSubject.Platform == nil {
							upsertSubject.Platform = &v1.Platform{}
						}

						upsertSubject.Platform.Architecture = platform.Architecture
					}

					if platform.Variant != "" {
						upsertConfig.Variant = platform.Variant
						if upsertSubject.Platform == nil {
							upsertSubject.Platform = &v1.Platform{}
						}

						upsertSubject.Platform.Variant = platform.Variant
					}

					if platform.OSVersion != "" {
						upsertConfig.OSVersion = platform.OSVersion
						if upsertSubject.Platform == nil {
							upsertSubject.Platform = &v1.Platform{}
						}

						upsertSubject.Platform.OSVersion = platform.OSVersion
					}

					if len(platform.Features) != 0 {
						plat := upsertConfig.Platform()
						if plat == nil {
							plat = &v1.Platform{}
						}

						plat.Features = append(plat.Features, platform.Features...)
						if upsertSubject.Platform == nil {
							upsertSubject.Platform = &v1.Platform{}
						}

						upsertSubject.Platform.Features = append(upsertSubject.Platform.Features, platform.Features...)
					}

					if len(platform.OSFeatures) != 0 {
						upsertConfig.OSFeatures = append(upsertConfig.OSFeatures, platform.OSFeatures...)
						if upsertSubject.Platform == nil {
							upsertSubject.Platform = &v1.Platform{}
						}

						upsertSubject.Platform.OSFeatures = append(upsertSubject.Platform.OSFeatures, platform.OSFeatures...)
					}

					ops = append(ops, layout.WithPlatform(*upsertSubject.Platform))
					img, err = mutate.ConfigFile(img, upsertConfig)
					if err != nil {
						return err
					}

					hash, err := img.Digest()
					if err != nil {
						return err
					}

					upsertSubject.Digest = hash
				}

				if len(desc.URLs) != 0 {
					upsertSubject.URLs = append(upsertSubject.URLs, desc.URLs...)
					ops = append(ops, layout.WithURLs(upsertSubject.URLs))
				}

				if len(desc.Annotations) != 0 {
					var annos = make(map[string]string)
					if len(upsertSubject.Annotations) != 0 {
						annos = upsertSubject.Annotations
					}

					for k, v := range desc.Annotations {
						annos[k] = v
					}

					upsertSubject.Annotations = annos
					ops = append(ops, layout.WithAnnotations(upsertSubject.Annotations))

					img = mutate.Annotations(img, upsertSubject.Annotations).(v1.Image)
					hash, err := img.Digest()
					if err != nil {
						return err
					}

					upsertSubject.Digest = hash
				}

				if len(ops) != 0 {
					img = mutate.Subject(img, *upsertSubject).(v1.Image)
				}

				iMap.Store(img, ops)
				return nil
			})

			if err = errGroup.Wait(); err != nil {
				return err
			}
		default:
			return ErrUnknownMediaType
		}
	}

	wg.Wait()
	i.Annotate = Annotate{
		Instance: make(map[v1.Hash]v1.Descriptor, 0),
	}
	iMap.Range(func(key, value any) bool {
		switch v := key.(type) {
		case v1.Image:
			ops, ok := value.([]layout.Option)
			if !ok {
				return false
			}

			err = path.AppendImage(v, ops...)
			if err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					Cause: err,
				})
			}
			return true
		case v1.ImageIndex:
			ops, ok := value.([]layout.Option)
			if !ok {
				return false
			}

			err = path.AppendIndex(v, ops...)
			if err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					Cause: err,
				})
			}
			return true
		default:
			return false
		}
	})

	if len(errs.Errors) != 0 {
		return errs
	}

	var removeHashes = make([]v1.Hash, 0)
	for _, h := range i.RemovedManifests {
		if _, ok := i.Images[h]; !ok {
			removeHashes = append(removeHashes, h)
			delete(i.Images, h)
		}
	}

	err = path.RemoveDescriptors(match.Digests(removeHashes...))
	if err != nil {
		return err
	}

	i.RemovedManifests = make([]v1.Hash, 0)
	return nil
}

// Publishes ImageIndex to the registry assuming every image it referes exists in registry.
//
// It will only push the IndexManifest to registry.
func (h *ManifestHandler) Push(ops ...IndexPushOption) error {
	if len(h.RemovedManifests) != 0 || len(h.Annotate.Instance) != 0 {
		return ErrIndexNeedToBeSaved
	}

	var pushOps = &PushOptions{}
	for _, op := range ops {
		err := op(pushOps)
		if err != nil {
			return err
		}
	}

	if pushOps.Format != types.MediaType("") {
		mfest, err := h.IndexManifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		if !pushOps.Format.IsIndex() {
			return ErrUnknownMediaType
		}

		if pushOps.Format != mfest.MediaType {
			h.ImageIndex = mutate.IndexMediaType(h.ImageIndex, pushOps.Format)
			if err := h.Save(); err != nil {
				return err
			}
		}
	}

	layoutPath := filepath.Join(h.Options.XdgPath, h.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		return err
	}

	h.ImageIndex, err = path.ImageIndex()
	if err != nil {
		return err
	}

	ref, err := name.ParseReference(
		h.Options.Reponame,
		name.WeakValidation,
		name.Insecure,
	)
	if err != nil {
		return err
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	taggableIndex := &TaggableIndex{
		IndexManifest: *mfest,
	}

	// Note: It will only push IndexManifest, assuming all the Images it refers exists in registry
	err = remote.Put(
		ref,
		taggableIndex,
		remote.WithAuthFromKeychain(h.Options.KeyChain),
		remote.WithTransport(getTransport(pushOps.Insecure)),
	)
	if err != nil {
		return err
	}

	if pushOps.Purge {
		return h.Delete()
	}

	return nil
}

// Publishes ImageIndex to the given Registry.
func (i *IndexHandler) Push(ops ...IndexPushOption) error {
	if len(i.RemovedManifests) != 0 || len(i.Annotate.Instance) != 0 {
		return ErrIndexNeedToBeSaved
	}

	var pushOps = &PushOptions{}
	for _, op := range ops {
		err := op(pushOps)
		if err != nil {
			return err
		}
	}

	if pushOps.Format != types.MediaType("") {
		mfest, err := i.IndexManifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		if !pushOps.Format.IsIndex() {
			return ErrUnknownMediaType
		}

		if pushOps.Format != mfest.MediaType {
			i.ImageIndex = mutate.IndexMediaType(i.ImageIndex, pushOps.Format)
			if err := i.Save(); err != nil {
				return err
			}
		}
	}

	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		return err
	}

	i.ImageIndex, err = path.ImageIndex()
	if err != nil {
		return err
	}

	mfest, err := i.IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	ref, err := name.ParseReference(
		i.Options.Reponame,
		name.WeakValidation,
		name.Insecure,
	)
	if err != nil {
		return err
	}

	var multiWriteTaggables = make(map[name.Reference]remote.Taggable)
	for _, desc := range mfest.Manifests {
		digest := ref.Context().Digest(desc.Digest.String())
		switch {
		case desc.MediaType.IsIndex():
			ii, err := i.ImageIndex.ImageIndex(desc.Digest)
			if err != nil {
				return err
			}

			multiWriteTaggables[digest] = ii
		case desc.MediaType.IsImage():
			img, err := i.Image(desc.Digest)
			if err != nil {
				return err
			}

			multiWriteTaggables[digest] = img
		default:
			return ErrUnknownMediaType
		}
	}

	// Push All the Images in ImageIndex as efficiently as possible, by deduping shared layer blobs while uploading them in parallel.
	err = remote.MultiWrite(
		multiWriteTaggables,
		remote.WithAuthFromKeychain(i.Options.KeyChain),
		remote.WithTransport(getTransport(pushOps.Insecure)),
	)
	if err != nil {
		return err
	}

	taggableIndex := &TaggableIndex{
		IndexManifest: *mfest,
	}

	// Push IndexManifest finally if every image it references exists in registry
	err = remote.Put(
		ref,
		taggableIndex,
		remote.WithAuthFromKeychain(i.Options.KeyChain),
		remote.WithTransport(getTransport(pushOps.Insecure)),
	)
	if err != nil {
		return err
	}

	if pushOps.Purge {
		return i.Delete()
	}

	return nil
}

// Displays IndexManifest.
func (h *ManifestHandler) Inspect() (string, error) {
	mfest, err := h.IndexManifest()
	if err != nil {
		return "", err
	}

	if mfest == nil {
		return "", ErrManifestUndefined
	}

	if len(h.RemovedManifests) != 0 || len(h.Annotate.Instance) != 0 {
		return "", ErrIndexNeedToBeSaved
	}

	mfestBytes, err := json.MarshalIndent(mfest, "", "	")
	if err != nil {
		return "", err
	}

	return string(mfestBytes), nil
}

// Displays IndexManifest.
func (i *IndexHandler) Inspect() (string, error) {
	mfest, err := i.IndexManifest()
	if err != nil {
		return "", err
	}

	if mfest == nil {
		return "", ErrManifestUndefined
	}

	if len(i.RemovedManifests) != 0 || len(i.Annotate.Instance) != 0 {
		return "", ErrIndexNeedToBeSaved
	}

	mfestBytes, err := json.MarshalIndent(mfest, "", "	")
	if err != nil {
		return "", err
	}

	return string(mfestBytes), nil
}

// Remove Image/Index from ImageIndex.
//
// Accepts both Tags and Digests.
func (h *ManifestHandler) Remove(ref name.Reference) (err error) {
	var hash v1.Hash
	switch v := ref.(type) {
	case name.Tag:
		desc, err := remote.Head(
			v,
			remote.WithAuthFromKeychain(h.Options.KeyChain),
			remote.WithTransport(
				getTransport(h.Options.InsecureRegistry),
			),
		)
		if err != nil {
			return err
		}

		if desc == nil {
			return ErrManifestUndefined
		}

		hash = desc.Digest
	default:
		hash, err = v1.NewHash(v.Identifier())
		if err != nil {
			return err
		}
	}

	if _, ok := h.Images[hash]; ok {
		h.RemovedManifests = append(h.RemovedManifests, hash)
		return nil
	}

	mfest, err := h.IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	found := false
	for _, d := range mfest.Manifests {
		if d.Digest == hash {
			found = true
			break
		}
	}

	if !found {
		return ErrNoImageOrIndexFoundWithGivenDigest
	}

	h.RemovedManifests = append(h.RemovedManifests, hash)
	return nil
}

// Remove Image/Index from ImageIndex.
//
// Accepts both Tags and Digests.
func (i *IndexHandler) Remove(ref name.Reference) (err error) {
	var hash v1.Hash
	switch v := ref.(type) {
	case name.Tag:
		desc, err := remote.Head(
			v,
			remote.WithAuthFromKeychain(i.Options.KeyChain),
			remote.WithTransport(
				getTransport(i.Options.InsecureRegistry),
			),
		)
		if err != nil {
			return err
		}

		if desc == nil {
			return ErrManifestUndefined
		}

		hash = desc.Digest
	default:
		hash, err = v1.NewHash(v.Identifier())
		if err != nil {
			return err
		}
	}

	if _, ok := i.Images[hash]; ok {
		i.RemovedManifests = append(i.RemovedManifests, hash)
		return nil
	}

	if _, err = i.ImageIndex.ImageIndex(hash); err != nil {
		_, err = i.Image(hash)
		if err != nil {
			return err
		}
	}

	i.RemovedManifests = append(i.RemovedManifests, hash)
	return nil
}

// Remove ImageIndex from local filesystem if exists.
func (h *ManifestHandler) Delete() error {
	layoutPath := filepath.Join(h.Options.XdgPath, h.Options.Reponame)
	if _, err := os.Stat(layoutPath); err != nil {
		return err
	}

	return os.RemoveAll(layoutPath)
}

// Remove ImageIndex from local filesystem if exists.
func (i *IndexHandler) Delete() error {
	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	if _, err := os.Stat(layoutPath); err != nil {
		return err
	}

	var wg sync.WaitGroup
	errGroup, _ := errgroup.WithContext(context.Background())
	filepath.WalkDir(layoutPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			wg.Add(1)
			errGroup.Go(func() error {
				defer wg.Done()
				return os.Remove(path)
			})
		}
		return nil
	})

	wg.Wait()
	return errGroup.Wait()
}

func getIndexURLs(i ImageIndex, hash v1.Hash) (urls []string, err error) {
	switch i := i.(type) {
	case *IndexHandler:
		idx, err := i.ImageIndex.ImageIndex(hash)
		if err != nil {
			return urls, err
		}

		mfest, err := idx.IndexManifest()
		if err != nil {
			return urls, err
		}

		if mfest == nil {
			return urls, ErrManifestUndefined
		}

		if mfest.Subject == nil {
			mfest.Subject = &v1.Descriptor{}
		}

		if len(mfest.Subject.URLs) == 0 {
			return urls, ErrURLsUndefined
		}

		return mfest.Subject.URLs, nil
	case *ManifestHandler:
		idx, err := i.ImageIndex.ImageIndex(hash)
		if err != nil {
			return urls, err
		}

		mfest, err := idx.IndexManifest()
		if err != nil {
			return urls, err
		}

		if mfest == nil {
			return urls, ErrManifestUndefined
		}

		if mfest.Subject == nil {
			mfest.Subject = &v1.Descriptor{}
		}

		if len(mfest.Subject.URLs) == 0 {
			return urls, ErrURLsUndefined
		}

		return mfest.Subject.URLs, nil
	default:
		return urls, ErrUnknownHandler
	}
}

func getImageURLs(i ImageIndex, hash v1.Hash) (urls []string, err error) {
	switch i := i.(type) {
	case *IndexHandler:
		if img, ok := i.Images[hash]; ok {
			return imageURLs(img)
		}

		img, err := i.Image(hash)
		if err != nil {
			return urls, err
		}

		return imageURLs(img)
	case *ManifestHandler:
		if desc, ok := i.Images[hash]; ok {
			if len(desc.URLs) == 0 {
				return urls, ErrURLsUndefined
			}

			return desc.URLs, nil
		}

		mfest, err := i.IndexManifest()
		if err != nil {
			return urls, err
		}

		if mfest == nil {
			return urls, ErrManifestUndefined
		}

		for _, desc := range mfest.Manifests {
			if desc.Digest == hash {
				if len(desc.URLs) == 0 {
					return urls, ErrURLsUndefined
				}

				return desc.URLs, nil
			}
		}

		return urls, ErrNoImageOrIndexFoundWithGivenDigest
	default:
		return urls, ErrUnknownHandler
	}
}

func imageURLs(img v1.Image) (urls []string, err error) {
	mfest, err := img.Manifest()
	if err != nil {
		return urls, err
	}

	if len(mfest.Config.URLs) != 0 {
		return mfest.Config.URLs, nil
	}

	if mfest.Subject == nil {
		mfest.Subject = &v1.Descriptor{}
	}

	if len(mfest.Subject.URLs) == 0 {
		return urls, ErrURLsUndefined
	}

	return mfest.Subject.URLs, nil
}

func getIndexManifest(i ImageIndex, digest name.Digest) (mfest *v1.IndexManifest, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	var indexManifest = func(idx v1.ImageIndex) (mfest *v1.IndexManifest, err error) {
		mfest, err = idx.IndexManifest()
		if err != nil {
			return
		}

		if mfest == nil {
			return mfest, ErrManifestUndefined
		}

		return mfest, err
	}

	switch i := i.(type) {
	case *IndexHandler:
		idx, err := i.ImageIndex.ImageIndex(hash)
		if err != nil {
			return nil, err
		}

		return indexManifest(idx)
	case *ManifestHandler:
		idx, err := i.ImageIndex.ImageIndex(hash)
		if err != nil {
			return nil, err
		}

		return indexManifest(idx)
	default:
		return nil, ErrUnknownHandler
	}
}

// Any ImageIndex with RawManifest method.
type TaggableIndex struct {
	v1.IndexManifest
}

// Returns the bytes of IndexManifest.
func (t *TaggableIndex) RawManifest() ([]byte, error) {
	return json.Marshal(t.IndexManifest)
}

// Returns the Digest of the IndexManifest if present.
// Else generate a new Digest.
func (t *TaggableIndex) Digest() (v1.Hash, error) {
	if t.IndexManifest.Subject != nil && t.IndexManifest.Subject.Digest != (v1.Hash{}) {
		return t.IndexManifest.Subject.Digest, nil
	}

	return partial.Digest(t)
}

// Returns the MediaType of the IndexManifest.
func (t *TaggableIndex) MediaType() (types.MediaType, error) {
	return t.IndexManifest.MediaType, nil
}

// Returns the Size of IndexManifest if present.
// Calculate the Size of empty.
func (t *TaggableIndex) Size() (int64, error) {
	if t.IndexManifest.Subject != nil && t.IndexManifest.Subject.Size != 0 {
		return t.IndexManifest.Subject.Size, nil
	}

	return partial.Size(t)
}
