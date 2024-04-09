package imgutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/sync/errgroup"
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
	ErrOSUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image os is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrArchUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image architecture is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrVariantUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image variant is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrOSVersionUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image os-version is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrFeaturesUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image features is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrOSFeaturesUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image os-features is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrURLsUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image urls is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrAnnotationsUndefined = func(format types.MediaType, digest string) error {
		return fmt.Errorf("Image annotations is undefined for %s ImageIndex (digest: %s)", indexMediaType(format), digest)
	}
	ErrNoImageOrIndexFoundWithGivenDigest = func(digest string) error {
		return fmt.Errorf(`no image or image index found for digest "%s"`, digest)
	}
	ErrConfigFilePlatformUndefined = errors.New("unable to determine image platform: ConfigFile's platform is nil")
	ErrManifestUndefined           = errors.New("encountered unexpected error while parsing image: manifest or index manifest is nil")
	ErrPlatformUndefined           = errors.New("unable to determine image platform: platform is nil")
	ErrInvalidPlatform             = errors.New("unable to determine image platform: platform's 'OS' or 'Architecture' field is nil")
	ErrConfigFileUndefined         = errors.New("unable to access image configuration: ConfigFile is nil")
	ErrIndexNeedToBeSaved          = errors.New(`unable to perform action: ImageIndex requires local storage before proceeding.
	Please use '#Save()' to save the image index locally before attempting this operation`)
	ErrUnknownMediaType = func(format types.MediaType) error {
		return fmt.Errorf("unsupported media type encountered in image: '%s'", format)
	}
	ErrNoImageFoundWithGivenPlatform = errors.New("no image found for specified platform")
)

var _ ImageIndex = (*ManifestHandler)(nil)

// A Handler implementing ImageIndex.
// Creates and Manipulate IndexManifest.
type ManifestHandler struct {
	v1.ImageIndex
	Annotate         Annotate
	Options          IndexOptions
	RemovedManifests []v1.Hash
	Images           map[v1.Hash]v1.Descriptor
}

func (h *ManifestHandler) getHash(digest name.Digest) (hash v1.Hash, err error) {
	if hash, err = v1.NewHash(digest.Identifier()); err != nil {
		return hash, err
	}

	// if any image is removed with given hash return an error
	for _, h := range h.RemovedManifests {
		if h == hash {
			return hash, ErrNoImageOrIndexFoundWithGivenDigest(h.String())
		}
	}

	return hash, nil
}

// Returns `OS` of an existing Image.
func (h *ManifestHandler) OS(digest name.Digest) (os string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return os, err
	}

	// if image is manipulated before return last manipulated value
	if os, err = h.Annotate.OS(hash); err == nil {
		return os, nil
	}

	getOS := func(desc v1.Descriptor) (os string, err error) {
		if desc.Platform == nil {
			return os, ErrPlatformUndefined
		}

		if desc.Platform.OS == "" {
			return os, ErrOSUndefined(desc.MediaType, hash.String())
		}

		return desc.Platform.OS, nil
	}

	// return the OS of the added image(using ImageIndex#Add) if found
	if desc, ok := h.Images[hash]; ok {
		return getOS(desc)
	}

	// check for the digest in the IndexManifest and return `OS` if found
	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return os, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			return getOS(desc)
		}
	}

	// when no image found with the given digest return an error
	return os, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Annotates existing Image by updating `OS` field in IndexManifest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetOS(digest name.Digest, os string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	// if any nested imageIndex found with given digest save underlying image instead of index with the given OS
	if mfest, err := h.getIndexManifest(digest); err == nil {
		// keep track of changes until ImageIndex#Save is called
		h.Annotate.SetOS(hash, os)
		h.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	// set the `OS` of an Image from base ImageIndex if found
	if img, err := h.Image(hash); err == nil {
		return h.setImageOS(img, hash, os)
	}

	// set the `OS` of an Image added to ImageIndex if found
	if desc, ok := h.Images[hash]; ok {
		// keep track of changes until ImageIndex#Save is called
		h.Annotate.SetOS(hash, os)
		h.Annotate.SetFormat(hash, desc.MediaType)

		return nil
	}

	// return an error if no Image found given digest
	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Add requested OS to `Annotate`
func (h *ManifestHandler) setImageOS(img v1.Image, hash v1.Hash, os string) error {
	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	h.Annotate.SetOS(hash, os)
	h.Annotate.SetFormat(hash, mfest.MediaType)
	return nil
}

// Return the Architecture of an Image/Index based on given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) Architecture(digest name.Digest) (arch string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return arch, err
	}

	if arch, err = h.Annotate.Architecture(hash); err == nil {
		return arch, nil
	}

	getArch := func(desc v1.Descriptor) (arch string, err error) {
		if desc.Platform == nil {
			return arch, ErrPlatformUndefined
		}

		if desc.Platform.Architecture == "" {
			return arch, ErrArchUndefined(desc.MediaType, hash.String())
		}

		return desc.Platform.Architecture, nil
	}

	if desc, ok := h.Images[hash]; ok {
		return getArch(desc)
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return arch, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			return getArch(desc)
		}
	}

	return arch, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Annotates the `Architecture` of an Image.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetArchitecture(digest name.Digest, arch string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	if mfest, err := h.getIndexManifest(digest); err == nil {
		h.Annotate.SetArchitecture(hash, arch)
		h.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return h.setImageArch(img, hash, arch)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetArchitecture(hash, arch)
		h.Annotate.SetFormat(hash, desc.MediaType)
		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Add request ARCH to `Annotate`
func (h *ManifestHandler) setImageArch(img v1.Image, hash v1.Hash, arch string) error {
	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	h.Annotate.SetArchitecture(hash, arch)
	h.Annotate.SetFormat(hash, mfest.MediaType)
	return nil
}

// Return the `Variant` of an Image.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) Variant(digest name.Digest) (osVariant string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return osVariant, err
	}

	if osVariant, err = h.Annotate.Variant(hash); err == nil {
		return osVariant, err
	}

	getVariant := func(desc v1.Descriptor) (osVariant string, err error) {
		if desc.Platform == nil {
			return osVariant, ErrPlatformUndefined
		}

		if desc.Platform.Variant == "" {
			return osVariant, ErrVariantUndefined(desc.MediaType, hash.String())
		}

		return desc.Platform.Variant, nil
	}

	if desc, ok := h.Images[hash]; ok {
		return getVariant(desc)
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return osVariant, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			return getVariant(desc)
		}
	}

	return osVariant, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Annotates the `Variant` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetVariant(digest name.Digest, osVariant string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	if mfest, err := h.getIndexManifest(digest); err == nil {
		h.Annotate.SetVariant(hash, osVariant)
		h.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return h.setImageVariant(img, hash, osVariant)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetVariant(hash, osVariant)
		h.Annotate.SetFormat(hash, desc.MediaType)
		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Add requested OSVariant to `Annotate`.
func (h *ManifestHandler) setImageVariant(img v1.Image, hash v1.Hash, osVariant string) error {
	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	h.Annotate.SetVariant(hash, osVariant)
	h.Annotate.SetFormat(hash, mfest.MediaType)
	return nil
}

// Returns the `OSVersion` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) OSVersion(digest name.Digest) (osVersion string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return osVersion, err
	}

	if osVersion, err = h.Annotate.OSVersion(hash); err == nil {
		return osVersion, nil
	}

	getOSVersion := func(desc v1.Descriptor) (osVersion string, err error) {
		if desc.Platform == nil {
			return osVersion, ErrPlatformUndefined
		}

		if desc.Platform.OSVersion == "" {
			return osVersion, ErrOSVersionUndefined(desc.MediaType, hash.String())
		}

		return desc.Platform.OSVersion, nil
	}

	if desc, ok := h.Images[hash]; ok {
		return getOSVersion(desc)
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return osVersion, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			return getOSVersion(desc)
		}
	}

	return osVersion, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Annotates the `OSVersion` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetOSVersion(digest name.Digest, osVersion string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	if mfest, err := h.getIndexManifest(digest); err == nil {
		h.Annotate.SetOSVersion(hash, osVersion)
		h.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return h.setImageOSVersion(img, hash, osVersion)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetOSVersion(hash, osVersion)
		h.Annotate.SetFormat(hash, desc.MediaType)
		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Add requested OSVersion to `Annotate`
func (h *ManifestHandler) setImageOSVersion(img v1.Image, hash v1.Hash, osVersion string) error {
	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	h.Annotate.SetOSVersion(hash, osVersion)
	h.Annotate.SetFormat(hash, mfest.MediaType)
	return nil
}

// Returns the `Features` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) Features(digest name.Digest) (features []string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return features, err
	}

	if features, err = h.Annotate.Features(hash); err == nil {
		return features, nil
	}

	if features, err = h.indexFeatures(digest); err == nil {
		return features, nil
	}

	getFeatures := func(desc v1.Descriptor) (features []string, err error) {
		if desc.Platform == nil {
			return features, ErrPlatformUndefined
		}

		if len(desc.Platform.Features) == 0 {
			return features, ErrFeaturesUndefined(desc.MediaType, hash.String())
		}

		var featuresSet = NewStringSet()
		for _, f := range desc.Platform.Features {
			featuresSet.Add(f)
		}

		return featuresSet.StringSlice(), nil
	}

	if desc, ok := h.Images[hash]; ok {
		return getFeatures(desc)
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return features, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			return getFeatures(desc)
		}
	}

	return features, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Returns Features from IndexManifest.
func (h *ManifestHandler) indexFeatures(digest name.Digest) (features []string, err error) {
	mfest, err := h.getIndexManifest(digest)
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
		return features, ErrFeaturesUndefined(mfest.MediaType, digest.Identifier())
	}

	return mfest.Subject.Platform.Features, nil
}

// Annotates the `Features` of an Image with given Digest by appending to existsing Features if any.
//
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetFeatures(digest name.Digest, features []string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	if mfest, err := h.getIndexManifest(digest); err == nil {
		h.Annotate.SetFeatures(hash, features)
		h.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return h.setImageFeatures(img, hash, features)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetFeatures(hash, features)
		h.Annotate.SetFormat(hash, desc.MediaType)
		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

func (h *ManifestHandler) setImageFeatures(img v1.Image, hash v1.Hash, features []string) error {
	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	h.Annotate.SetFeatures(hash, features)
	h.Annotate.SetFormat(hash, mfest.MediaType)
	return nil
}

// Returns the `OSFeatures` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return osFeatures, err
	}

	if osFeatures, err = h.Annotate.OSFeatures(hash); err == nil {
		return osFeatures, nil
	}

	osFeatures, err = h.indexOSFeatures(digest)
	if err == nil {
		return osFeatures, nil
	}

	getOSFeatures := func(desc v1.Descriptor) (osFeatures []string, err error) {
		if desc.Platform == nil {
			return osFeatures, ErrPlatformUndefined
		}

		if len(desc.Platform.OSFeatures) == 0 {
			return osFeatures, ErrOSFeaturesUndefined(desc.MediaType, digest.Identifier())
		}

		var osFeaturesSet = NewStringSet()
		for _, s := range desc.Platform.OSFeatures {
			osFeaturesSet.Add(s)
		}

		return osFeaturesSet.StringSlice(), nil
	}

	if desc, ok := h.Images[hash]; ok {
		return getOSFeatures(desc)
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return osFeatures, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			return getOSFeatures(desc)
		}
	}

	return osFeatures, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Returns OSFeatures from IndexManifest.
func (h *ManifestHandler) indexOSFeatures(digest name.Digest) (osFeatures []string, err error) {
	mfest, err := h.getIndexManifest(digest)
	if err != nil {
		return osFeatures, err
	}

	if mfest.Subject == nil {
		mfest.Subject = &v1.Descriptor{}
	}

	if mfest.Subject.Platform == nil {
		mfest.Subject.Platform = &v1.Platform{}
	}

	if len(mfest.Subject.Platform.OSFeatures) == 0 {
		return osFeatures, ErrOSFeaturesUndefined(mfest.MediaType, digest.Identifier())
	}

	return mfest.Subject.Platform.OSFeatures, nil
}

// Annotates the `OSFeatures` of an Image with given Digest by appending to existsing OSFeatures if any.
//
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	if mfest, err := h.getIndexManifest(digest); err == nil {
		h.Annotate.SetOSFeatures(hash, osFeatures)
		h.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return h.setImageOSFeatures(img, hash, osFeatures)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetOSFeatures(hash, osFeatures)
		h.Annotate.SetFormat(hash, desc.MediaType)
		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

func (h *ManifestHandler) setImageOSFeatures(img v1.Image, hash v1.Hash, osFeatures []string) error {
	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	h.Annotate.SetOSFeatures(hash, osFeatures)
	h.Annotate.SetFormat(hash, mfest.MediaType)
	return nil
}

// Return the `Annotations` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
//
// For Docker Images and Indexes it returns an error.
func (h *ManifestHandler) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return annotations, err
	}

	getAnnotations := func(annos map[string]string, format types.MediaType) (map[string]string, error) {
		switch format {
		case types.DockerManifestSchema2,
			types.DockerManifestSchema1,
			types.DockerManifestSchema1Signed,
			types.DockerManifestList:
			// Docker Manifest doesn't support annotations
			return nil, ErrAnnotationsUndefined(format, digest.Identifier())
		case types.OCIManifestSchema1,
			types.OCIImageIndex:
			if len(annos) == 0 {
				return nil, ErrAnnotationsUndefined(format, digest.Identifier())
			}

			return annos, nil
		default:
			return annos, ErrUnknownMediaType(format)
		}
	}

	if annotations, err = h.Annotate.Annotations(hash); err == nil {
		format, err := h.Annotate.Format(hash)
		if err != nil {
			return annotations, err
		}

		return getAnnotations(annotations, format)
	}

	annotations, format, err := h.indexAnnotations(digest)
	if err == nil || errors.Is(err, ErrAnnotationsUndefined(format, digest.Identifier())) {
		return annotations, err
	}

	if desc, ok := h.Images[hash]; ok {
		return getAnnotations(desc.Annotations, desc.MediaType)
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return annotations, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			return getAnnotations(desc.Annotations, desc.MediaType)
		}
	}

	return annotations, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

func (h *ManifestHandler) indexAnnotations(digest name.Digest) (annotations map[string]string, format types.MediaType, err error) {
	mfest, err := h.getIndexManifest(digest)
	if err != nil {
		return
	}

	if len(mfest.Annotations) == 0 {
		return annotations, types.DockerConfigJSON, ErrAnnotationsUndefined(mfest.MediaType, digest.Identifier())
	}

	if mfest.MediaType == types.DockerManifestList {
		return nil, types.DockerManifestList, ErrAnnotationsUndefined(mfest.MediaType, digest.Identifier())
	}

	return mfest.Annotations, types.OCIImageIndex, nil
}

// Annotates the `Annotations` of an Image with given Digest by appending to existsing Annotations if any.
//
// Returns an error if no Image/Index found with given Digest.
//
// For Docker Images and Indexes it ignores updating Annotations.
func (h *ManifestHandler) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
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

	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Returns the `URLs` of an Image with given Digest.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) URLs(digest name.Digest) (urls []string, err error) {
	hash, err := h.getHash(digest)
	if err != nil {
		return urls, err
	}

	if urls, err = h.Annotate.URLs(hash); err == nil {
		var urlSet = NewStringSet()
		for _, s := range urls {
			urlSet.Add(s)
		}
		return urlSet.StringSlice(), nil
	}

	if urls, err = h.getIndexURLs(hash); err == nil {
		return urls, nil
	}

	urls, format, err := h.getImageURLs(hash)
	if err == nil {
		return urls, nil
	}

	if err == ErrURLsUndefined(format, digest.Identifier()) {
		return urls, ErrURLsUndefined(format, digest.Identifier())
	}

	return urls, ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Annotates the `URLs` of an Image with given Digest by appending to existsing URLs if any.
// Returns an error if no Image/Index found with given Digest.
func (h *ManifestHandler) SetURLs(digest name.Digest, urls []string) error {
	hash, err := h.getHash(digest)
	if err != nil {
		return err
	}

	if mfest, err := h.getIndexManifest(digest); err == nil {
		h.Annotate.SetURLs(hash, urls)
		h.Annotate.SetFormat(hash, mfest.MediaType)
		return nil
	}

	if img, err := h.Image(hash); err == nil {
		return h.setImageURLs(img, hash, urls)
	}

	if desc, ok := h.Images[hash]; ok {
		h.Annotate.SetURLs(hash, urls)
		h.Annotate.SetFormat(hash, desc.MediaType)
		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier())
}

// Adds the requested URLs to `Annotate`.
func (h *ManifestHandler) setImageURLs(img v1.Image, hash v1.Hash, urls []string) error {
	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	h.Annotate.SetURLs(hash, urls)
	h.Annotate.SetFormat(hash, mfest.MediaType)
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

	layoutPath := filepath.Join(h.Options.XdgPath, MakeFileSafeName(h.Options.Reponame))
	path, pathErr := layout.FromPath(layoutPath)
	if addOps.Local {
		if pathErr != nil {
			return pathErr
		}
		img := addOps.Image
		var (
			os, _          = img.OS()
			arch, _        = img.Architecture()
			variant, _     = img.Variant()
			osVersion, _   = img.OSVersion()
			features, _    = img.Features()
			osFeatures, _  = img.OSFeatures()
			urls, _        = img.URLs()
			annos, _       = img.Annotations()
			size, _        = img.ManifestSize()
			mediaType, err = img.MediaType()
			digest, _      = img.Digest()
		)
		if err != nil {
			return err
		}

		desc := v1.Descriptor{
			MediaType:   mediaType,
			Size:        size,
			Digest:      digest,
			URLs:        urls,
			Annotations: annos,
			Platform: &v1.Platform{
				OS:           os,
				Architecture: arch,
				Variant:      variant,
				OSVersion:    osVersion,
				Features:     features,
				OSFeatures:   osFeatures,
			},
		}

		return path.AppendDescriptor(desc)
	}

	// Fetch Descriptor of the given reference.
	//
	// This call is returns a v1.Descriptor with `Size`, `MediaType`, `Digest` fields only!!
	// This is a light weight call used for checking MediaType of given Reference
	desc, err := remote.Head(
		ref,
		remote.WithAuthFromKeychain(h.Options.KeyChain),
		remote.WithTransport(GetTransport(h.Options.Insecure())),
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
			remote.WithTransport(GetTransport(h.Options.Insecure())),
		)
		if err != nil {
			return err
		}

		mfest, err := getManifest(img)
		if err != nil {
			return err
		}

		imgConfig, err := getConfigFile(img)
		if err != nil {
			return err
		}

		platform := v1.Platform{}
		if err := updatePlatform(imgConfig, &platform); err != nil {
			return err
		}

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

		if pathErr != nil {
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
			idx, err := remote.Index(
				ref,
				remote.WithAuthFromKeychain(h.Options.KeyChain),
				remote.WithTransport(GetTransport(h.Options.Insecure())),
			)
			if err != nil {
				return err
			}

			var iMap sync.Map
			errs := SaveError{}
			// Add all the Images from Nested ImageIndexes
			if err = h.addAllImages(idx, addOps.Annotations, &iMap); err != nil {
				return err
			}

			if err != nil {
				// if the ImageIndex is not saved till now for some reason Save the ImageIndex locally to append Images
				if err = h.Save(); err != nil {
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
				if err = path.AppendDescriptor(desc); err != nil {
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
			return h.addPlatformSpecificImages(ref, *platformSpecificDesc, addOps.Annotations)
		default:
			platform := v1.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
			}

			// Add the Image from the ImageIndex with current Device's Platform
			return h.addPlatformSpecificImages(ref, platform, addOps.Annotations)
		}
	default:
		// return an error if the Reference is neither an Image not an Index
		return ErrUnknownMediaType(desc.MediaType)
	}
}

func (h *ManifestHandler) addAllImages(idx v1.ImageIndex, annotations map[string]string, imageMap *sync.Map) error {
	mfest, err := getIndexManifest(idx)
	if err != nil {
		return err
	}

	var errs, _ = errgroup.WithContext(context.Background())
	for _, desc := range mfest.Manifests {
		desc := desc
		errs.Go(func() error {
			return h.addIndexAddendum(annotations, desc, idx, imageMap)
		})
	}

	return errs.Wait()
}

func (h *ManifestHandler) addIndexAddendum(annotations map[string]string, desc v1.Descriptor, idx v1.ImageIndex, iMap *sync.Map) error {
	switch {
	case desc.MediaType.IsIndex():
		ii, err := idx.ImageIndex(desc.Digest)
		if err != nil {
			return err
		}

		return h.addAllImages(ii, annotations, iMap)
	case desc.MediaType.IsImage():
		img, err := idx.Image(desc.Digest)
		if err != nil {
			return err
		}

		mfest, err := getManifest(img)
		if err != nil {
			return err
		}

		imgConfig, err := img.ConfigFile()
		if err != nil {
			return err
		}

		platform := v1.Platform{}
		if err = updatePlatform(imgConfig, &platform); err != nil {
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

		h.Images[desc.Digest] = *config
		iMap.Store(desc.Digest, *config)

		return nil
	default:
		return ErrUnknownMediaType(desc.MediaType)
	}
}

func (h *ManifestHandler) addPlatformSpecificImages(ref name.Reference, platform v1.Platform, annotations map[string]string) error {
	if platform.OS == "" || platform.Architecture == "" {
		return ErrInvalidPlatform
	}

	desc, err := remote.Get(
		ref,
		remote.WithAuthFromKeychain(h.Options.KeyChain),
		remote.WithTransport(GetTransport(true)),
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

	mfest, err := getManifest(img)
	if err != nil {
		return err
	}

	imgConfig, err := getConfigFile(img)
	if err != nil {
		return err
	}

	platform = v1.Platform{}
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

	h.Images[digest] = *config

	layoutPath := filepath.Join(h.Options.XdgPath, MakeFileSafeName(h.Options.Reponame))
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		if path, err = layout.Write(layoutPath, h.ImageIndex); err != nil {
			return err
		}
	}

	return path.AppendDescriptor(*config)
}

// Save IndexManifest locally.
// Use it save manifest locally iff the manifest doesn't exist locally before
func (h *ManifestHandler) save(layoutPath string) (path layout.Path, err error) {
	// If the ImageIndex is not saved before Save the ImageIndex
	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return path, err
	}

	// Initially write an empty IndexManifest with expected MediaType
	if mfest.MediaType == types.OCIImageIndex {
		if path, err = layout.Write(layoutPath, empty.Index); err != nil {
			return path, err
		}
	} else {
		if path, err = layout.Write(layoutPath, NewEmptyDockerIndex()); err != nil {
			return path, err
		}
	}

	// loop over each digest and append Image/ImageIndex
	for _, d := range mfest.Manifests {
		switch {
		case d.MediaType.IsIndex(), d.MediaType.IsImage():
			if err = path.AppendDescriptor(d); err != nil {
				return path, err
			}
		default:
			return path, ErrUnknownMediaType(d.MediaType)
		}
	}

	return path, nil
}

// Save will locally save the given ImageIndex.
func (h *ManifestHandler) Save() error {
	layoutPath := filepath.Join(h.Options.XdgPath, MakeFileSafeName(h.Options.Reponame))
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		if path, err = h.save(layoutPath); err != nil {
			return err
		}
	}

	hashes := make([]v1.Hash, 0, len(h.Annotate.Instance))
	for h := range h.Annotate.Instance {
		hashes = append(hashes, h)
	}

	// Remove all the Annotated Images/ImageIndexes from local ImageIndex to avoid duplicate Images with same Digest
	if err = path.RemoveDescriptors(match.Digests(hashes...)); err != nil {
		return err
	}

	var errs SaveError
	for hash, desc := range h.Annotate.Instance {
		// If the digest matches an Image added annotate the Image and Save Locally
		if imgDesc, ok := h.Images[hash]; ok {
			if !imgDesc.MediaType.IsImage() && !imgDesc.MediaType.IsIndex() {
				return ErrUnknownMediaType(imgDesc.MediaType)
			}

			appendAnnotatedManifests(desc, imgDesc, path, &errs)
			continue
		}

		// Using IndexManifest annotate required changes
		mfest, err := getIndexManifest(h.ImageIndex)
		if err != nil {
			return err
		}

		var imageFound = false
		for _, imgDesc := range mfest.Manifests {
			if imgDesc.Digest == hash {
				imageFound = true
				if !imgDesc.MediaType.IsImage() && !imgDesc.MediaType.IsIndex() {
					return ErrUnknownMediaType(imgDesc.MediaType)
				}

				appendAnnotatedManifests(desc, imgDesc, path, &errs)
				break
			}
		}

		if !imageFound {
			return ErrNoImageOrIndexFoundWithGivenDigest(hash.String())
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

// Publishes ImageIndex to the registry assuming every image it referes exists in registry.
//
// It will only push the IndexManifest to registry.
func (h *ManifestHandler) Push(ops ...IndexPushOption) error {
	if len(h.RemovedManifests) != 0 || len(h.Annotate.Instance) != 0 {
		return ErrIndexNeedToBeSaved
	}

	var pushOps = &PushOptions{}
	for _, op := range ops {
		if err := op(pushOps); err != nil {
			return err
		}
	}

	if pushOps.Format != types.MediaType("") {
		mfest, err := getIndexManifest(h.ImageIndex)
		if err != nil {
			return err
		}

		if !pushOps.Format.IsIndex() {
			return ErrUnknownMediaType(pushOps.Format)
		}

		if pushOps.Format != mfest.MediaType {
			h.ImageIndex = mutate.IndexMediaType(h.ImageIndex, pushOps.Format)
			if err := h.Save(); err != nil {
				return err
			}
		}
	}

	layoutPath := filepath.Join(h.Options.XdgPath, MakeFileSafeName(h.Options.Reponame))
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		return err
	}

	if h.ImageIndex, err = path.ImageIndex(); err != nil {
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

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return err
	}

	var taggableIndex = NewTaggableIndex(mfest)
	multiWriteTagables := map[name.Reference]remote.Taggable{
		ref: taggableIndex,
	}
	for _, tag := range pushOps.Tags {
		multiWriteTagables[ref.Context().Tag(tag)] = taggableIndex
	}

	// Note: It will only push IndexManifest, assuming all the Images it refers exists in registry
	err = remote.MultiWrite(
		multiWriteTagables,
		remote.WithAuthFromKeychain(h.Options.KeyChain),
		remote.WithTransport(GetTransport(pushOps.Insecure)),
	)

	if pushOps.Purge {
		return h.Delete()
	}

	return err
}

// Displays IndexManifest.
func (h *ManifestHandler) Inspect() (string, error) {
	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return "", err
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

// Remove Image/Index from ImageIndex.
//
// Accepts both Tags and Digests.
func (h *ManifestHandler) Remove(ref name.Reference) (err error) {
	hash, err := parseReferenceToHash(ref, h.Options)
	if err != nil {
		return err
	}

	if _, ok := h.Images[hash]; ok {
		h.RemovedManifests = append(h.RemovedManifests, hash)
		return nil
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		return err
	}

	found := false
	for _, d := range mfest.Manifests {
		if d.Digest == hash {
			found = true
			break
		}
	}

	if !found {
		return ErrNoImageOrIndexFoundWithGivenDigest(ref.Identifier())
	}

	h.RemovedManifests = append(h.RemovedManifests, hash)
	return nil
}

// Remove ImageIndex from local filesystem if exists.
func (h *ManifestHandler) Delete() error {
	layoutPath := filepath.Join(h.Options.XdgPath, MakeFileSafeName(h.Options.Reponame))
	if _, err := os.Stat(layoutPath); err != nil {
		return err
	}

	return os.RemoveAll(layoutPath)
}

func (h *ManifestHandler) getIndexURLs(hash v1.Hash) (urls []string, err error) {
	idx, err := h.ImageIndex.ImageIndex(hash)
	if err != nil {
		return urls, err
	}

	mfest, err := getIndexManifest(idx)
	if err != nil {
		return urls, err
	}

	if mfest.Subject == nil {
		mfest.Subject = &v1.Descriptor{}
	}

	if len(mfest.Subject.URLs) == 0 {
		return urls, ErrURLsUndefined(mfest.MediaType, hash.String())
	}

	return mfest.Subject.URLs, nil
}

func (h *ManifestHandler) getImageURLs(hash v1.Hash) (urls []string, format types.MediaType, err error) {
	if desc, ok := h.Images[hash]; ok {
		if len(desc.URLs) == 0 {
			return urls, desc.MediaType, ErrURLsUndefined(desc.MediaType, hash.String())
		}

		return desc.URLs, desc.MediaType, nil
	}

	mfest, err := getIndexManifest(h.ImageIndex)
	if err != nil {
		// Return Non-Image and Non-Index mediaType
		return urls, types.DockerConfigJSON, err
	}

	for _, desc := range mfest.Manifests {
		if desc.Digest == hash {
			if len(desc.URLs) == 0 {
				return urls, desc.MediaType, ErrURLsUndefined(desc.MediaType, hash.String())
			}

			return desc.URLs, desc.MediaType, nil
		}
	}

	return urls, mfest.MediaType, ErrNoImageOrIndexFoundWithGivenDigest(hash.String())
}

func (h *ManifestHandler) getIndexManifest(digest name.Digest) (mfest *v1.IndexManifest, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if mfest, err = getIndexManifest(h.ImageIndex); err != nil {
		return mfest, err
	}

	for _, desc := range mfest.Manifests {
		desc := desc
		if desc.Digest == hash {
			return &v1.IndexManifest{
				MediaType: desc.MediaType,
				Subject:   &desc,
			}, nil
		}
	}

	return nil, ErrNoImageOrIndexFoundWithGivenDigest(hash.String())
}

// Change a reference name string into a valid file name
// Ex: cnbs/sample-package:hello-multiarch-universe
// to cnbs_sample-package-hello-multiarch-universe
func MakeFileSafeName(ref string) string {
	fileName := strings.ReplaceAll(ref, ":", "-")
	return strings.ReplaceAll(fileName, "/", "_")
}