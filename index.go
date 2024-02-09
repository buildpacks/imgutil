package imgutil

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/sync/errgroup"
)

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
	Inspect() error
	Remove(digest name.Digest) error
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
)

var _ ImageIndex = (*Index)(nil)

type Index struct {
	v1.ImageIndex
	Annotate         Annotate
	Options          IndexOptions
	RemovedManifests []v1.Hash
	Images           map[v1.Hash]v1.Image
}

type Annotate struct {
	Instance map[v1.Hash]v1.Descriptor
}

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

func (i *Index) OS(digest name.Digest) (os string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return os, ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if os, err = i.Annotate.OS(hash); err == nil {
		return
	}

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

func (i *Index) SetOS(digest name.Digest, os string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(*i, digest); err == nil {
		i.Annotate.SetOS(hash, os)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	if img, err := i.Image(hash); err == nil {
		return imageSetOS(i, img, hash, os)
	}

	if img, ok := i.Images[hash]; ok {
		return imageSetOS(i, img, hash, os)
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func imageSetOS(i *Index, img v1.Image, hash v1.Hash, os string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	i.Annotate.SetOS(hash, os)
	i.Annotate.SetFormat(hash, mfest.MediaType)

	return nil
}

func (i *Index) Architecture(digest name.Digest) (arch string, err error) {
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

func (i *Index) SetArchitecture(digest name.Digest, arch string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(*i, digest); err == nil {
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

func imageSetArch(i *Index, img v1.Image, hash v1.Hash, arch string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	i.Annotate.SetArchitecture(hash, arch)
	i.Annotate.SetFormat(hash, mfest.MediaType)

	return nil
}

func (i *Index) Variant(digest name.Digest) (osVariant string, err error) {
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

func (i *Index) SetVariant(digest name.Digest, osVariant string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(*i, digest); err == nil {
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

func imageSetVariant(i *Index, img v1.Image, hash v1.Hash, osVariant string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	i.Annotate.SetVariant(hash, osVariant)
	i.Annotate.SetFormat(hash, mfest.MediaType)

	return nil
}

func (i *Index) OSVersion(digest name.Digest) (osVersion string, err error) {
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

func (i *Index) SetOSVersion(digest name.Digest, osVersion string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(*i, digest); err == nil {
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

func imageSetOSVersion(i *Index, img v1.Image, hash v1.Hash, osVersion string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	i.Annotate.SetOSVersion(hash, osVersion)
	i.Annotate.SetFormat(hash, mfest.MediaType)

	return nil
}

func (i *Index) Features(digest name.Digest) (features []string, err error) {
	var indexFeatures = func(i *Index, digest name.Digest) (features []string, err error) {
		mfest, err := getIndexManifest(*i, digest)
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

func (i *Index) SetFeatures(digest name.Digest, features []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(*i, digest); err == nil {
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

func imageSetFeatures(i *Index, img v1.Image, hash v1.Hash, features []string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	i.Annotate.SetFeatures(hash, features)
	i.Annotate.SetFormat(hash, mfest.MediaType)

	return nil
}

func (i *Index) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	var indexOSFeatures = func(i *Index, digest name.Digest) (osFeatures []string, err error) {
		mfest, err := getIndexManifest(*i, digest)
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

func (i *Index) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(*i, digest); err == nil {
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

func imageSetOSFeatures(i *Index, img v1.Image, hash v1.Hash, osFeatures []string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	i.Annotate.SetOSFeatures(hash, osFeatures)
	i.Annotate.SetFormat(hash, mfest.MediaType)

	return nil
}

func (i *Index) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	var indexAnnotations = func(i *Index, digest name.Digest) (annotations map[string]string, err error) {
		mfest, err := getIndexManifest(*i, digest)
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

func (i *Index) SetAnnotations(digest name.Digest, annotations map[string]string) error {
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

func imageSetAnnotations(i *Index, img v1.Image, hash v1.Hash, annotations map[string]string) error {
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

	i.Annotate.SetAnnotations(hash, annos)
	i.Annotate.SetFormat(hash, mfest.MediaType)
	return nil
}

func (i *Index) URLs(digest name.Digest) (urls []string, err error) {
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

func (i *Index) SetURLs(digest name.Digest, urls []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.RemovedManifests {
		if h == hash {
			return ErrNoImageOrIndexFoundWithGivenDigest
		}
	}

	if mfest, err := getIndexManifest(*i, digest); err == nil {
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

func imageSetURLs(i *Index, img v1.Image, hash v1.Hash, urls []string) error {
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	i.Annotate.SetURLs(hash, urls)
	i.Annotate.SetFormat(hash, mfest.MediaType)

	return nil
}

func (i *Index) Add(ref name.Reference, ops ...IndexAddOption) error {
	var addOps = &AddOptions{}
	for _, op := range ops {
		op(addOps)
	}

	desc, err := remote.Get(
		ref,
		remote.WithAuthFromKeychain(i.Options.KeyChain),
		remote.WithTransport(getTransport(i.Options.Insecure())),
	)

	if err != nil {
		return err
	}

	switch {
	case desc.MediaType.IsImage():
		img, err := desc.Image()
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
		annos := mfest.Annotations
		if desc.MediaType == types.OCIManifestSchema1 && len(addOps.Annotations) != 0 {
			if len(annos) == 0 {
				annos = make(map[string]string)
			}

			for k, v := range addOps.Annotations {
				annos[k] = v
			}

			layoutOps = append(layoutOps, layout.WithAnnotations(annos))
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

		i.Images[desc.Digest] = img
		return path.AppendImage(img, layoutOps...)
	case desc.MediaType.IsIndex():
		idx, err := desc.ImageIndex()
		if err != nil {
			return err
		}

		switch {
		case addOps.All:
			var wg sync.WaitGroup
			var imageMap sync.Map
			errs := SaveError{}

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

			return addPlatformSpecificImages(i, ref, *platformSpecificDesc, addOps.Annotations)
		default:
			platform := v1.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
			}

			return addPlatformSpecificImages(i, ref, platform, addOps.Annotations)
		}
	default:
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

func addAllImages(i *Index, idx *v1.ImageIndex, annotations map[string]string, wg *sync.WaitGroup, imageMap *sync.Map) error {
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

func addIndexAddendum(i *Index, annotations map[string]string, desc v1.Descriptor, idx *v1.ImageIndex, wg *sync.WaitGroup, iMap *sync.Map) error {
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
			// i.Annotate.SetAnnotations(desc.Digest, annos)
			// i.Annotate.SetFormat(desc.Digest, desc.MediaType)
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
}

func addPlatformSpecificImages(i *Index, ref name.Reference, platform v1.Platform, annotations map[string]string) error {
	if platform.OS == "" || platform.Architecture == "" {
		return ErrInvalidPlatform
	}

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
}

func appendImage(i *Index, desc *remote.Descriptor, annotations map[string]string) error {
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

func (i *Index) Save() error {
	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		path, err = layout.Write(layoutPath, i.ImageIndex)
		if err != nil {
			return err
		}
	}

	hashes := make([]v1.Hash, 0, len(i.Annotate.Instance))
	for h := range i.Annotate.Instance {
		hashes = append(hashes, h)
	}

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
					ii = mutate.Subject(mutate.Annotations(ii, annos).(v1.ImageIndex), *upsertSubject).(v1.ImageIndex)
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
	i.Annotate = Annotate{}
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

func (i *Index) Push(ops ...IndexPushOption) error {
	var pushOps = &PushOptions{}

	if len(i.RemovedManifests) != 0 || len(i.Annotate.Instance) != 0 {
		if err := i.Save(); err != nil {
			return err
		}
	}

	for _, op := range ops {
		err := op(pushOps)
		if err != nil {
			return err
		}
	}

	ref, err := name.ParseReference(
		i.Options.Reponame,
		name.WeakValidation,
		name.Insecure,
	)
	if err != nil {
		return err
	}

	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		return err
	}

	imageIndex, err := path.ImageIndex()
	if err != nil {
		return err
	}

	if pushOps.Format != "" {
		mfest, err := i.IndexManifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		if pushOps.Format != types.MediaType("") && pushOps.Format != mfest.MediaType {
			imageIndex = mutate.IndexMediaType(imageIndex, pushOps.Format)
		}
	}

	err = remote.WriteIndex(
		ref,
		imageIndex,
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

func (i *Index) Inspect() error {
	mfest, err := i.IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	if len(i.RemovedManifests) != 0 || len(i.Annotate.Instance) != 0 {
		return ErrIndexNeedToBeSaved
	}

	mfestBytes, err := json.MarshalIndent(mfest, "", "		")
	if err != nil {
		return err
	}

	return errors.New(string(mfestBytes))
}

func (i *Index) Remove(digest name.Digest) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
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

func (i *Index) Delete() error {
	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	if _, err := os.Stat(layoutPath); err != nil {
		return err
	}

	return os.RemoveAll(layoutPath)
}

func getIndexURLs(i *Index, hash v1.Hash) (urls []string, err error) {
	idx, err := i.ImageIndex.ImageIndex(hash)
	if err != nil {
		return
	}

	mfest, err := idx.IndexManifest()
	if err != nil {
		return
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
}

func getImageURLs(i *Index, hash v1.Hash) (urls []string, err error) {
	if img, ok := i.Images[hash]; ok {
		return imageURLs(img)
	}

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	return imageURLs(img)
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

func getConfigFile(img v1.Image) (config *v1.ConfigFile, err error) {
	config, err = img.ConfigFile()
	if err != nil {
		return
	}

	if config == nil {
		return config, ErrConfigFileUndefined
	}

	return config, nil
}

func getIndexManifest(i Index, digest name.Digest) (mfest *v1.IndexManifest, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	idx, err := i.ImageIndex.ImageIndex(hash)
	if err != nil {
		return
	}

	mfest, err = idx.IndexManifest()
	if err != nil {
		return
	}

	if mfest == nil {
		return mfest, ErrManifestUndefined
	}

	return mfest, err
}
