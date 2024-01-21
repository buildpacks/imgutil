package imgutil

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil/docker"
)

const digestDelim = "@"

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
)

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
	desc, ok := a.Instance[hash]
	if !ok || desc.Platform == nil || desc.Platform.OS == "" {
		return os, ErrOSUndefined
	}

	return desc.Platform.OS, nil
}

func (a *Annotate) SetOS(hash v1.Hash, os string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OS = os
	a.Instance[hash] = desc
}

func (a *Annotate) Architecture(hash v1.Hash) (arch string, err error) {
	desc := a.Instance[hash]
	if desc.Platform == nil || desc.Platform.Architecture == "" {
		return arch, ErrArchUndefined
	}

	return desc.Platform.Architecture, nil
}

func (a *Annotate) SetArchitecture(hash v1.Hash, arch string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Architecture = arch
	a.Instance[hash] = desc
}

func (a *Annotate) Variant(hash v1.Hash) (variant string, err error) {
	desc := a.Instance[hash]
	if desc.Platform == nil || desc.Platform.Variant == "" {
		return variant, ErrVariantUndefined
	}

	return desc.Platform.Variant, nil
}

func (a *Annotate) SetVariant(hash v1.Hash, variant string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Variant = variant
	a.Instance[hash] = desc
}

func (a *Annotate) OSVersion(hash v1.Hash) (osVersion string, err error) {
	desc := a.Instance[hash]
	if desc.Platform == nil || desc.Platform.OSVersion == "" {
		return osVersion, ErrOSVersionUndefined
	}

	return desc.Platform.OSVersion, nil
}

func (a *Annotate) SetOSVersion(hash v1.Hash, osVersion string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OSVersion = osVersion
	a.Instance[hash] = desc
}

func (a *Annotate) Features(hash v1.Hash) (features []string, err error) {
	desc := a.Instance[hash]
	if desc.Platform == nil || len(desc.Platform.Features) == 0 {
		return features, ErrFeaturesUndefined
	}

	return desc.Platform.Features, nil
}

func (a *Annotate) SetFeatures(hash v1.Hash, features []string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Features = features
	a.Instance[hash] = desc
}

func (a *Annotate) OSFeatures(hash v1.Hash) (osFeatures []string, err error) {
	desc := a.Instance[hash]
	if desc.Platform == nil || len(desc.Platform.OSFeatures) == 0 {
		return osFeatures, ErrOSFeaturesUndefined
	}

	return desc.Platform.OSFeatures, nil
}

func (a *Annotate) SetOSFeatures(hash v1.Hash, osFeatures []string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OSFeatures = osFeatures
	a.Instance[hash] = desc
}

func (a *Annotate) Annotations(hash v1.Hash) (annotations map[string]string, err error) {
	desc := a.Instance[hash]
	if len(desc.Annotations) == 0 {
		return annotations, ErrAnnotationsUndefined
	}

	return desc.Annotations, nil
}

func (a *Annotate) SetAnnotations(hash v1.Hash, annotations map[string]string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Annotations = annotations
	a.Instance[hash] = desc
}

func (a *Annotate) URLs(hash v1.Hash) (urls []string, err error) {
	desc := a.Instance[hash]
	if len(desc.URLs) == 0 {
		return urls, ErrURLsUndefined
	}

	return desc.URLs, nil
}

func (a *Annotate) SetURLs(hash v1.Hash, urls []string) {
	desc := a.Instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.URLs = urls
	a.Instance[hash] = desc
}

func (a *Annotate) Format(hash v1.Hash) (format types.MediaType, err error) {
	desc := a.Instance[hash]
	if desc.MediaType == "" {
		return format, ErrUnknownMediaType
	}

	return desc.MediaType, nil
}

func (a *Annotate) SetFormat(hash v1.Hash, format types.MediaType) {
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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetOS(hash, os)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetOS(hash, os)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetArchitecture(hash, arch)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetArchitecture(hash, arch)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

		return nil
	}

	return ErrUnknownMediaType
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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetVariant(hash, osVariant)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetVariant(hash, osVariant)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

		return nil
	}

	return ErrUnknownMediaType
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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetOSVersion(hash, osVersion)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetOSVersion(hash, osVersion)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

		return nil
	}

	return ErrUnknownMediaType
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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetFeatures(hash, features)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetFeatures(hash, features)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

		return nil
	}

	return ErrUnknownMediaType
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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetOSFeatures(hash, osFeatures)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetOSFeatures(hash, osFeatures)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

		return nil
	}

	return ErrUnknownMediaType
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
			return nil, nil
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
		return
	}

	annotations, err = indexAnnotations(i, digest)
	if err == nil {
		return
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

	if mfest.MediaType == types.DockerManifestSchema2 {
		return nil, nil
	}

	return mfest.Annotations, nil
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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetAnnotations(hash, annotations)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetAnnotations(hash, annotations)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

		return nil
	}

	return ErrUnknownMediaType
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

	if _, err = i.ImageIndex.ImageIndex(hash); err == nil {
		i.Annotate.SetURLs(hash, urls)
		i.Annotate.SetFormat(hash, types.OCIImageIndex)

		return nil
	}

	if _, err = i.Image(hash); err == nil {
		i.Annotate.SetURLs(hash, urls)
		i.Annotate.SetFormat(hash, types.OCIManifestSchema1)

		return nil
	}

	return ErrUnknownMediaType
}

func (i *Index) Add(ref name.Reference, ops ...IndexAddOption) error {
	var addOps = &AddOptions{}
	for _, op := range ops {
		if err := op(addOps); err != nil {
			return err
		}
	}

	var fetchPlatformSpecificImage = false

	platform := v1.Platform{}

	if addOps.os != "" {
		platform.OS = addOps.os
		fetchPlatformSpecificImage = true
	}

	if addOps.arch != "" {
		platform.Architecture = addOps.arch
		fetchPlatformSpecificImage = true
	}

	if addOps.variant != "" {
		platform.Variant = addOps.variant
		fetchPlatformSpecificImage = true
	}

	if addOps.osVersion != "" {
		platform.OSVersion = addOps.osVersion
		fetchPlatformSpecificImage = true
	}

	if len(addOps.features) != 0 {
		platform.Features = addOps.features
		fetchPlatformSpecificImage = true
	}

	if len(addOps.osFeatures) != 0 {
		platform.OSFeatures = addOps.osFeatures
		fetchPlatformSpecificImage = true
	}

	if fetchPlatformSpecificImage {
		return addPlatformSpecificImages(i, ref, platform, addOps.annotations)
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

		var desc v1.Descriptor
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		if mfest.Subject != nil && mfest.Subject.Platform != nil {
			desc = *mfest.Subject
		}

		if mfest.Config.Platform != nil {
			desc = mfest.Config
		}

		if reflect.DeepEqual(desc, v1.Descriptor{}) {
			desc = mfest.Config
		}

		i.ImageIndex = mutate.AppendManifests(i.ImageIndex, mutate.IndexAddendum{
			Add:        img,
			Descriptor: desc,
		})

		return nil
	case desc.MediaType.IsIndex():
		idx, err := desc.ImageIndex()
		if err != nil {
			return err
		}

		if addOps.all {
			return addAllImages(i, idx, ref, addOps.annotations)
		}

		platform := v1.Platform{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
		}

		return addPlatformSpecificImages(i, ref, platform, addOps.annotations)
	default:
		return ErrNoImageOrIndexFoundWithGivenDigest
	}
}

func addAllImages(i *Index, idx v1.ImageIndex, ref name.Reference, annotations map[string]string) error {
	mfest, err := idx.IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	errs := SaveError{}

	for _, desc := range mfest.Manifests {
		if desc.MediaType.IsIndex() {
			err := addImagesFromDigest(i, desc.Digest, ref, annotations)
			if err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					ImageName: desc.Digest.String(),
					Cause:     err,
				})
			}
		}
	}

	if len(errs.Errors) == 0 {
		return nil
	}

	return errors.New(errs.Error())
}

func addImagesFromDigest(i *Index, hash v1.Hash, ref name.Reference, annotations map[string]string) error {
	imgRef, err := name.ParseReference(ref.Context().Name() + digestDelim + hash.String())
	if err != nil {
		return err
	}

	desc, err := remote.Get(
		imgRef,
		remote.WithAuthFromKeychain(i.Options.KeyChain),
		remote.WithTransport(getTransport(true)),
	)
	if err != nil {
		return err
	}

	switch {
	case desc.MediaType.IsImage():
		return appendImage(i, desc, annotations)
	case desc.MediaType.IsIndex():
		idx, err := desc.ImageIndex()
		if err != nil {
			return err
		}

		return addAllImages(i, idx, ref, annotations)
	default:
		return ErrNoImageOrIndexFoundWithGivenDigest
	}
}

func addPlatformSpecificImages(i *Index, ref name.Reference, platform v1.Platform, annotations map[string]string) error {
	if platform.OS == "" {
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

	return addImage(i, &img, annotations)
}

func addImage(i *Index, img *v1.Image, annotations map[string]string) error {
	var v1Desc = v1.Descriptor{}
	mfest, err := (*img).Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return ErrManifestUndefined
	}

	if mfest.Subject != nil && mfest.Subject.Platform != nil {
		v1Desc = *mfest.Subject
	}

	if mfest.Config.Platform != nil {
		v1Desc = mfest.Config
	}

	if reflect.DeepEqual(v1Desc, v1.Descriptor{}) {
		v1Desc = mfest.Config
	}

	if reflect.DeepEqual(v1Desc, v1.Descriptor{}) {
		return ErrConfigFileUndefined
	}

	if len(annotations) != 0 {
		v1Desc.Annotations = annotations
	}

	i.ImageIndex = mutate.AppendManifests(i.ImageIndex, mutate.IndexAddendum{
		Add:        *img,
		Descriptor: v1Desc,
	})

	return nil
}

func (i *Index) Save() error {
	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	if _, err := os.Stat(filepath.Join(layoutPath, "index.json")); err != nil {
		format, err := i.MediaType()
		if err != nil {
			return err
		}

		switch format {
		case types.DockerManifestList:
			_, err = layout.Write(layoutPath, docker.DockerIndex)
			if err != nil {
				return err
			}
		case types.OCIImageIndex:
			_, err = layout.Write(layoutPath, empty.Index)
			if err != nil {
				return err
			}
		default:
			return errors.New(ErrUnknownMediaType.Error() + fmt.Sprintf("; found %s", format))
		}
	}

	path, err := layout.FromPath(layoutPath)
	if err != nil {
		return err
	}

	var errs = SaveError{}
	for hash, desc := range i.Annotate.Instance {
		switch {
		case desc.MediaType.IsImage():
			img, err := i.Image(hash)
			if err != nil {
				return err
			}

			var upsertDesc = v1.Descriptor{}
			mfest, err := img.Manifest()
			if err != nil {
				return err
			}

			if mfest == nil {
				return ErrManifestUndefined
			}

			upsertDesc = mfest.Config
			if mfest.Subject != nil {
				upsertDesc = *mfest.Subject.DeepCopy()
			}

			if upsertDesc.Platform == nil {
				upsertDesc.Platform = &v1.Platform{}
			}

			var ops = []layout.Option{}
			if desc.Platform != nil {
				if desc.Platform.OS != "" {
					upsertDesc.Platform.OS = desc.Platform.OS
				}

				if desc.Platform.Architecture != "" {
					upsertDesc.Platform.Architecture = desc.Platform.Architecture
				}

				if desc.Platform.Variant != "" {
					upsertDesc.Platform.Variant = desc.Platform.Variant
				}

				if desc.Platform.OSVersion != "" {
					upsertDesc.Platform.OSVersion = desc.Platform.OSVersion
				}

				if len(desc.Platform.Features) != 0 {
					upsertDesc.Platform.Features = desc.Platform.Features
				}

				if len(desc.Platform.OSFeatures) != 0 {
					upsertDesc.Platform.OSFeatures = desc.Platform.OSFeatures
				}

				ops = append(ops, layout.WithPlatform(*upsertDesc.Platform))
			}

			if mfest.MediaType == types.DockerManifestSchema2 ||
				mfest.MediaType == types.DockerManifestSchema1 ||
				mfest.MediaType == types.DockerManifestSchema1Signed {
				ops = append(ops, layout.WithAnnotations(map[string]string(nil)))
			} else if len(desc.Annotations) != 0 {
				ops = append(ops, layout.WithAnnotations(desc.Annotations))
			}

			if len(desc.URLs) != 0 {
				ops = append(ops, layout.WithURLs(desc.URLs))
			}

			err = path.ReplaceImage(img, match.Digests(hash), ops...)
			if err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					ImageName: hash.String(),
					Cause:     err,
				})
			}
		case desc.MediaType.IsIndex():
			idx, err := i.ImageIndex.ImageIndex(hash)
			if err != nil {
				return err
			}

			var upsertDesc = v1.Descriptor{}
			mfest, err := idx.IndexManifest()
			if err != nil {
				return err
			}

			if mfest == nil {
				return ErrManifestUndefined
			}

			if mfest.Subject != nil {
				return ErrManifestUndefined
			}

			upsertDesc = *mfest.Subject
			if upsertDesc.Platform == nil {
				upsertDesc.Platform = &v1.Platform{}
			}

			var ops = []layout.Option{}
			if desc.Platform != nil {
				if desc.Platform.OS != "" {
					upsertDesc.Platform.OS = desc.Platform.OS
				}

				if desc.Platform.Architecture != "" {
					upsertDesc.Platform.Architecture = desc.Platform.Architecture
				}

				if desc.Platform.Variant != "" {
					upsertDesc.Platform.Variant = desc.Platform.Variant
				}

				if desc.Platform.OSVersion != "" {
					upsertDesc.Platform.OSVersion = desc.Platform.OSVersion
				}

				if len(desc.Platform.Features) != 0 {
					upsertDesc.Platform.Features = desc.Platform.Features
				}

				if len(desc.Platform.OSFeatures) != 0 {
					upsertDesc.Platform.OSFeatures = desc.Platform.OSFeatures
				}

				ops = append(ops, layout.WithPlatform(*upsertDesc.Platform))
			}

			if len(desc.Annotations) != 0 && mfest.MediaType != types.DockerManifestList {
				ops = append(ops, layout.WithAnnotations(desc.Annotations))
			}

			if len(desc.URLs) != 0 {
				ops = append(ops, layout.WithURLs(desc.URLs))
			}

			err = path.ReplaceIndex(idx, match.Digests(hash), ops...)
			if err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					ImageName: hash.String(),
					Cause:     err,
				})
			}
		default:
			return errors.New(ErrUnknownMediaType.Error() + fmt.Sprintf("; found %v", desc.MediaType))
		}
	}

	i.Annotate = Annotate{}
	for _, h := range i.RemovedManifests {
		err = path.RemoveDescriptors(match.Digests(h))
		if err != nil {
			errs.Errors = append(errs.Errors, SaveDiagnostic{
				ImageName: h.String(),
				Cause:     err,
			})
		}
	}

	i.RemovedManifests = []v1.Hash{}
	if len(errs.Errors) != 0 {
		return errors.New(errs.Error())
	}

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

	if pushOps.format != "" {
		mfest, err := i.IndexManifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return ErrManifestUndefined
		}

		if pushOps.format != mfest.MediaType {
			imageIndex = mutate.IndexMediaType(imageIndex, pushOps.format)
		}
	}

	err = remote.WriteIndex(
		ref,
		imageIndex,
		remote.WithAuthFromKeychain(i.Options.KeyChain),
		remote.WithTransport(getTransport(pushOps.insecure)),
	)
	if err != nil {
		return err
	}

	if pushOps.purge {
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
