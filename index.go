package imgutil

import (
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	config, err := getConfigFile(img)
	if err != nil {
		return
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
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		i.Annotate.SetOS(hash, os)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
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

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	config, err := getConfigFile(img)
	if err != nil {
		return
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
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		i.Annotate.SetArchitecture(hash, arch)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
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

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	config, err := getConfigFile(img)
	if err != nil {
		return
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
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		i.Annotate.SetVariant(hash, osVariant)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
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

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	config, err := getConfigFile(img)
	if err != nil {
		return
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
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		i.Annotate.SetOSVersion(hash, osVersion)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
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

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	config, err := getConfigFile(img)
	if err != nil {
		return
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
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		i.Annotate.SetFeatures(hash, features)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
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

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	config, err := getConfigFile(img)
	if err != nil {
		return
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
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		i.Annotate.SetOSFeatures(hash, osFeatures)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
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

	img, err := i.Image(hash)
	if err != nil {
		return
	}

	mfest, err := img.Manifest()
	if err != nil {
		return
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
		mfest, err := img.Manifest()
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

	return ErrNoImageOrIndexFoundWithGivenDigest
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
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		i.Annotate.SetURLs(hash, urls)
		i.Annotate.SetFormat(hash, mfest.MediaType)

		return nil
	}

	return ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) Add(ref name.Reference, ops ...IndexAddOption) error {
	var addOps = &AddOptions{}
	for _, op := range ops {
		op(addOps)
	}

	desc, err := remote.Get(
		ref,
		remote.WithAuthFromKeychain(i.Options.KeyChain),
		remote.WithTransport(getTransport(true)),
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

		i.ImageIndex = mutate.AppendManifests(
			i.ImageIndex,
			mutate.IndexAddendum{
				Add: img,
			},
		)

		if desc.MediaType == types.OCIManifestSchema1 {
			annos := desc.Annotations
			if len(annos) == 0 {
				annos = make(map[string]string)
			}

			for k, v := range addOps.Annotations {
				annos[k] = v
			}

			i.Annotate.SetAnnotations(desc.Digest, annos)
			i.Annotate.SetFormat(desc.Digest, desc.MediaType)
		}

		return nil
	case desc.MediaType.IsIndex():
		idx, err := desc.ImageIndex()
		if err != nil {
			return err
		}

		switch {
		case addOps.All:
			return addAllImages(i, &idx, addOps.Annotations)
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

func addAllImages(i *Index, idx *v1.ImageIndex, annotations map[string]string) error {
	mfest, err := (*idx).IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	errs := SaveError{}
	addEndums := []mutate.IndexAddendum{}
	for _, desc := range mfest.Manifests {
		addEndums, err = addIndexAddendum(i, annotations, desc, addEndums, idx)
		if err != nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				ImageName: desc.Digest.String(),
				Cause:     err,
			})
		}
	}

	i.ImageIndex = mutate.AppendManifests(*idx, addEndums...)
	if len(errs.Errors) != 0 {
		return errors.New(errs.Error())
	}

	return nil
}

func addIndexAddendum(i *Index, annotations map[string]string, desc v1.Descriptor, addEndums []mutate.IndexAddendum, idx *v1.ImageIndex) ([]mutate.IndexAddendum, error) {
	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	path, err := layout.FromPath(layoutPath)
	if err != nil {
		err = i.Save()
		if err != nil {
			return addEndums, err
		}
	}

	switch {
	case desc.MediaType.IsIndex():
		ii, err := (*idx).ImageIndex(desc.Digest)
		if err != nil {
			return addEndums, err
		}

		return addEndums, addAllImages(i, &ii, annotations)
	case desc.MediaType.IsImage():
		img, err := (*idx).Image(desc.Digest)
		if err != nil {
			return addEndums, err
		}

		mfest, err := img.Manifest()
		if err != nil {
			return addEndums, err
		}

		if mfest == nil {
			return addEndums, ErrManifestUndefined
		}

		if mfest.Subject == nil {
			mfest.Subject = &v1.Descriptor{}
		}

		var ops []layout.Option
		if len(annotations) != 0 {
			var annos = make(map[string]string)
			if len(mfest.Config.Annotations) != 0 {
				annos = mfest.Config.Annotations
			}

			if mfest.Subject != nil {
				if len(mfest.Subject.Annotations) != 0 {
					annos = mfest.Subject.Annotations
				}
			}

			for k, v := range annotations {
				annos[k] = v
			}

			ops = append(ops, layout.WithAnnotations(annos))
			i.Annotate.SetAnnotations(desc.Digest, annos)
			i.Annotate.SetFormat(desc.Digest, desc.MediaType)
		}

		addEndums = append(
			addEndums,
			mutate.IndexAddendum{
				Add: img,
			},
		)
		return addEndums, path.AppendImage(img, ops...)
	default:
		return addEndums, ErrUnknownMediaType
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

	annos := desc.Annotations
	if len(annos) == 0 {
		annos = make(map[string]string)
	}

	for k, v := range annotations {
		annos[k] = v
	}

	i.Annotate.SetAnnotations(digest, annos)
	i.Annotate.SetFormat(digest, desc.MediaType)

	i.ImageIndex = mutate.AppendManifests(
		i.ImageIndex,
		mutate.IndexAddendum{
			Add: img,
		},
	)
	return nil
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

	for hash, desc := range i.Annotate.Instance {
		switch {
		case desc.MediaType.IsIndex():
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
			if len(desc.Annotations) != 0 {
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

			err = path.AppendIndex(ii, ops...)
			if err != nil {
				return err
			}
		case desc.MediaType.IsImage():
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

			if upsertSubject.Platform.OS == "" {
				upsertSubject.Platform.OS = config.OS
			}

			if upsertSubject.Platform.Architecture == "" {
				upsertSubject.Platform.Architecture = config.Architecture
			}

			if upsertSubject.Platform.Variant == "" {
				upsertSubject.Platform.Variant = config.Variant
			}

			if upsertSubject.Platform.OSVersion == "" {
				upsertSubject.Platform.OSVersion = config.OSVersion
			}

			if len(upsertSubject.Platform.Features) == 0 {
				if platform := config.Platform(); platform != nil && len(platform.Features) != 0 {
					upsertSubject.Platform.Features = platform.Features
				}
			}

			if len(upsertSubject.Platform.OSFeatures) == 0 {
				if len(upsertConfig.OSFeatures) != 0 {
					upsertSubject.Platform.OSFeatures = config.OSFeatures
				}
			}

			if platform := desc.Platform; platform != nil && !reflect.DeepEqual(*platform, v1.Platform{}) {
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

			err = path.AppendImage(img, ops...)
			if err != nil {
				return err
			}
		default:
			return ErrUnknownMediaType
		}
	}
	i.Annotate = Annotate{}

	err = path.RemoveDescriptors(match.Digests(i.RemovedManifests...))
	if err != nil {
		return err
	}

	i.RemovedManifests = make([]v1.Hash, 0)
	return nil
}

func (i *Index) Push(ops ...IndexPushOption) error {
	var imageIndex = i.ImageIndex
	var pushOps = &PushOptions{}

	if len(i.RemovedManifests) != 0 || len(i.Annotate.Instance) != 0 {
		return ErrIndexNeedToBeSaved
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

	if pushOps.Format != "" {
		mfest, err := i.IndexManifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		if pushOps.Format != mfest.MediaType {
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
	bytes, err := i.RawManifest()
	if err != nil {
		return err
	}

	if len(i.RemovedManifests) != 0 || len(i.Annotate.Instance) != 0 {
		return ErrIndexNeedToBeSaved
	}

	return errors.New(string(bytes))
}

func (i *Index) Remove(digest name.Digest) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
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
	return os.RemoveAll(filepath.Join(i.Options.XdgPath, i.Options.Reponame))
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
	img, err := i.Image(hash)
	if err != nil {
		return
	}

	mfest, err := img.Manifest()
	if err != nil {
		return
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

func getTransport(insecure bool) http.RoundTripper {
	// #nosec G402
	if insecure {
		return &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return http.DefaultTransport
}
