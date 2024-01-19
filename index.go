package imgutil

import (
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

type Index struct {
	v1.ImageIndex
	annotate Annotate
	Options IndexOptions
	removedManifests []v1.Hash
}

type Annotate struct {
	instance map[v1.Hash]v1.Descriptor
}

func(a *Annotate) OS(hash v1.Hash) (os string, err error) {
	desc := a.instance[hash]
	if desc.Platform == nil || desc.Platform.OS == "" {
		return os, errors.New("os is undefined")
	}

	return desc.Platform.OS, nil
}

func(a *Annotate) SetOS(hash v1.Hash, os string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OS = os
	a.instance[hash] = desc
}

func(a *Annotate) Architecture(hash v1.Hash) (arch string, err error) {
	desc := a.instance[hash]
	if desc.Platform == nil || desc.Platform.Architecture == "" {
		return arch, errors.New("architecture is undefined")
	}

	return desc.Platform.Architecture, nil
}

func(a *Annotate) SetArchitecture(hash v1.Hash, arch string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Architecture = arch
	a.instance[hash] = desc
}

func(a *Annotate) Variant(hash v1.Hash) (variant string, err error) {
	desc := a.instance[hash]
	if desc.Platform == nil || desc.Platform.Variant == "" {
		return variant, errors.New("variant is undefined")
	}

	return desc.Platform.Variant, nil
}

func(a *Annotate) SetVariant(hash v1.Hash, variant string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Variant = variant
	a.instance[hash] = desc
}

func(a *Annotate) OSVersion(hash v1.Hash) (osVersion string, err error) {
	desc := a.instance[hash]
	if desc.Platform == nil || desc.Platform.OSVersion == "" {
		return osVersion, errors.New("osVersion is undefined")
	}

	return desc.Platform.OSVersion, nil
}

func(a *Annotate) SetOSVersion(hash v1.Hash, osVersion string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OSVersion = osVersion
	a.instance[hash] = desc
}

func(a *Annotate) Features(hash v1.Hash) (features []string, err error) {
	desc := a.instance[hash]
	if desc.Platform == nil || len(desc.Platform.Features) == 0 {
		return features, errors.New("features is undefined")
	}

	return desc.Platform.Features, nil
}

func(a *Annotate) SetFeatures(hash v1.Hash, features []string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.Features = features
	a.instance[hash] = desc
}

func(a *Annotate) OSFeatures(hash v1.Hash) (osFeatures []string, err error) {
	desc := a.instance[hash]
	if desc.Platform == nil || len(desc.Platform.OSFeatures) == 0 {
		return osFeatures, errors.New("osFeatures is undefined")
	}

	return desc.Platform.OSFeatures, nil
}

func(a *Annotate) SetOSFeatures(hash v1.Hash, osFeatures []string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OSFeatures = osFeatures
	a.instance[hash] = desc
}

func(a *Annotate) Annotations(hash v1.Hash) (annotations map[string]string, err error) {
	desc := a.instance[hash]
	if len(desc.Annotations) == 0 {
		return annotations, errors.New("annotations is undefined")
	}

	return desc.Annotations, nil
}

func(a *Annotate) SetAnnotations(hash v1.Hash, annotations map[string]string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Annotations = annotations
	a.instance[hash] = desc
}

func(a *Annotate) URLs(hash v1.Hash) (urls []string, err error) {
	desc := a.instance[hash]
	if len(desc.URLs) == 0 {
		return urls, errors.New("urls are undefined")
	}

	return desc.URLs, nil
}

func(a *Annotate) SetURLs(hash v1.Hash, urls []string) {
	desc := a.instance[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.URLs = urls
	a.instance[hash] = desc
}

func (i *Index) OS(digest name.Digest) (os string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return os, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if os, err = i.annotate.OS(hash); err == nil {
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
		return os, errors.New("os is undefined")
	}

	return config.OS, nil
}

func(i *Index) SetOS(digest name.Digest, os string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetOS(hash, os)

	return nil
}

func (i *Index) Architecture(digest name.Digest) (arch string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return arch, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if arch, err = i.annotate.Architecture(hash); err == nil {
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
		return arch, errors.New("architecture is undefined")
	}

	return config.Architecture, nil
}

func(i *Index) SetArchitecture(digest name.Digest, arch string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetArchitecture(hash, arch)

	return nil
}

func (i *Index) Variant(digest name.Digest) (osVariant string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return osVariant, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if osVariant, err = i.annotate.Variant(hash); err == nil {
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
		return osVariant, errors.New("variant is undefined")
	}

	return config.Variant, nil
}

func(i *Index) SetVariant(digest name.Digest, osVariant string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetVariant(hash, osVariant)

	return nil
}

func (i *Index) OSVersion(digest name.Digest) (osVersion string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return osVersion, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if osVersion, err = i.annotate.OSVersion(hash); err == nil {
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
		return osVersion, errors.New("osVersion is undefined")
	}

	return config.OSVersion, nil
}

func(i *Index) SetOSVersion(digest name.Digest, osVersion string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetOSVersion(hash, osVersion)

	return nil
}

func (i *Index) Features(digest name.Digest) (features []string, err error) {
	var indexFeatures = func (i *Index, digest name.Digest) (features []string, err error)  {
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
			return features, errors.New("features is undefined")
		}

		return mfest.Subject.Platform.Features, nil
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return features, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if features, err = i.annotate.Features(hash); err == nil {
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
		return features, errors.New("config platform is undefined")
	}

	if len(platform.Features) == 0 {
		return features, errors.New("features undefined")
	}

	return platform.Features, nil
}

func(i *Index) SetFeatures(digest name.Digest, features []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetFeatures(hash, features)

	return nil
}

func(i *Index) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
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
			return osFeatures, errors.New("os features is undefined")
		}

		return mfest.Subject.Platform.OSFeatures, nil
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return osFeatures, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if osFeatures, err = i.annotate.OSFeatures(hash); err == nil {
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
		return osFeatures, errors.New("osFeatures are undefined")
	}

	return config.OSFeatures, nil
}

func(i *Index) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetOSFeatures(hash, osFeatures)

	return nil
}

func(i *Index) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	var indexAnnotations = func(i *Index, digest name.Digest) (annotations map[string]string, err error) {
		mfest, err := getIndexManifest(*i, digest)
		if err != nil {
			return
		}

		if len(mfest.Annotations) == 0 {
			return annotations, errors.New("annotations are undefined")
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

	for _, h := range i.removedManifests {
		if h == hash {
			return annotations, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if annotations, err = i.annotate.Annotations(hash); err == nil {
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

	if mfest == nil || len(mfest.Annotations) == 0 {
		return annotations, errors.New("manifest is undefined")
	}

	if mfest.MediaType == types.DockerManifestSchema2 {
		return nil, nil
	}

	return mfest.Annotations, nil
}

func(i *Index) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetAnnotations(hash, annotations)

	return nil
}

func(i *Index) URLs(digest name.Digest) (urls []string, err error) {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return urls, errors.New("image/index with the given digest doesn't exists")
		}
	}

	if urls, err = i.annotate.URLs(hash); err == nil {
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

	return urls, errors.New("no image or image index found with the given digest")
}

func(i *Index) SetURLs(digest name.Digest, urls []string) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	for _, h := range i.removedManifests {
		if h == hash {
			return errors.New("image/index with the given digest doesn't exists")
		}
	}

	i.annotate.SetURLs(hash, urls)

	return nil
}

func(i *Index) Add(ref name.Reference, ops ...IndexAddOption) error {
	var addOps = &AddOptions{}
	for _, op := range ops {
		if err := op(addOps); err != nil {
			return err
		}
	}

	var fetchPlatformSpecificImage bool = false

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

	switch{
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
			return errors.New("image manifest doesn't exists")
		}

		if mfest.Subject != nil && mfest.Subject.Platform != nil {
			desc = *mfest.Subject
		} else if mfest.Config.Platform != nil {
			desc = mfest.Config
		} else {
			desc = mfest.Config
		}

		i.ImageIndex = mutate.AppendManifests(i.ImageIndex, mutate.IndexAddendum{
			Add: img,
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
			OS: runtime.GOOS,
			Architecture: runtime.GOARCH,
		}

		return addPlatformSpecificImages(i, ref, platform, addOps.annotations)
	default:
		return errors.New("cannot find image/image index with the given reference")
	}
}

func addAllImages(i *Index, idx v1.ImageIndex, ref name.Reference, annotations map[string]string) error {
	mfest, err := idx.IndexManifest()
	if err != nil {
		return err
	}
	
	if mfest == nil {
		return errors.New("index manifest is undefined")
	}

	errs := SaveError{}

	for _, desc := range mfest.Manifests {
		if desc.MediaType.IsIndex() {
			err := addImagesFromDigest(i, desc.Digest, ref, annotations)
			if err != nil {
				errs.Errors = append(errs.Errors, SaveDiagnostic{
					ImageName: desc.Digest.String(),
					Cause: err,
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

	desc, err := remote.Get(imgRef, remote.WithAuthFromKeychain(i.Options.KeyChain), remote.WithTransport(getTransport(true)))
	if err != nil {
		return err
	}

	switch{
	case desc.MediaType.IsImage():
		return appendImage(i, desc, annotations)
	case desc.MediaType.IsIndex():
		idx, err := desc.ImageIndex()
		if err != nil {
			return  err
		}

		return addAllImages(i, idx, ref, annotations)
	default:
		return errors.New("no image/image index found with the given hash: "+ hash.String())
	}
}

func addPlatformSpecificImages(i *Index, ref name.Reference, platform v1.Platform, annotations map[string]string) error {
	if platform.OS == "" {
		return errors.New("error fetching image from index with unknown platform")
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

	return addImage(i, img, annotations)
}

func addImage(i *Index, img v1.Image, annotations map[string]string) error {
	var v1Desc v1.Descriptor
	mfest, err := img.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return errors.New("image manifest doesn't exists")
	}

	if mfest.Subject != nil && mfest.Subject.Platform != nil {
		v1Desc = *mfest.Subject
	} else if mfest.Config.Platform != nil {
		v1Desc = mfest.Config
	} else {
		v1Desc = mfest.Config
	}

	if len(annotations) != 0 {
		v1Desc.Annotations = annotations
	}

	i.ImageIndex = mutate.AppendManifests(i.ImageIndex, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1Desc,
	})

	return nil
}

func(i *Index) Save() error {
	for hash, desc := range i.annotate.instance {
		img, err := i.Image(hash)
		if err != nil {
			return err
		}
	
		config, _ := getConfigFile(img)
		if config == nil {
			config = &v1.ConfigFile{}
		}

		platform, _ := getConfigFilePlatform(*config)
		mfest, err := img.Manifest()
		if err != nil {
			return err
		}

		var imgDesc v1.Descriptor
		if mfest.Config.Platform != nil {
			imgDesc = mfest.Config
		} else if mfest.Subject != nil && mfest.Subject.Platform != nil {
			imgDesc = *mfest.Subject
		} else if mfest.Subject != nil && mfest.Subject.Platform == nil {
			mfest.Subject.Platform = platform
			imgDesc = *mfest.Subject
		} else if mfest.Config.Platform == nil {
			mfest.Config.Platform = platform
			imgDesc = mfest.Config
		} else {
			imgDesc.Platform = &v1.Platform{}
		}

		upsertDesc := desc.DeepCopy()
		if upsertDesc.Platform != nil {
			if upsertDesc.Platform.OS != "" {
				imgDesc.Platform.OS = upsertDesc.Platform.OS
			}

			if upsertDesc.Platform.Architecture != "" {
				imgDesc.Platform.Architecture = upsertDesc.Platform.Architecture
			}

			if upsertDesc.Platform.Variant != "" {
				imgDesc.Platform.Variant = upsertDesc.Platform.Variant
			}

			if upsertDesc.Platform.OSVersion != "" {
				imgDesc.Platform.OSVersion = upsertDesc.Platform.OSVersion
			}

			if len(upsertDesc.Platform.Features) != 0 {
				imgDesc.Platform.Features = upsertDesc.Platform.Features
			}

			if len(upsertDesc.Platform.OSFeatures) != 0 {
				imgDesc.Platform.OSFeatures = upsertDesc.Platform.OSFeatures
			}
		}

		if len(upsertDesc.Annotations) != 0 {
			imgDesc.Annotations = upsertDesc.Annotations
		}

		if len(upsertDesc.URLs) != 0 {
			imgDesc.URLs = upsertDesc.URLs
		}

		i.ImageIndex = mutate.AppendManifests(
			mutate.RemoveManifests(
				i.ImageIndex, 
				match.Digests(hash),
			), mutate.IndexAddendum{
				Add: img,
				Descriptor: imgDesc,
			},
		)
	}

	i.annotate = Annotate{}
	for _, h := range i.removedManifests {
		i.ImageIndex = mutate.RemoveManifests(i.ImageIndex, match.Digests(h))
	}
	i.removedManifests = []v1.Hash{}

	layoutPath := filepath.Join(i.Options.XdgPath, i.Options.Reponame)
	if _, err := os.Stat(filepath.Join(layoutPath, "index.json")); err != nil {
		path := layout.Path(layoutPath)
		return path.WriteIndex(i.ImageIndex)
	}

	path, err := layout.FromPath(layoutPath)
	if err != nil {
		return err
	}

	return path.WriteIndex(i.ImageIndex)
}

func(i *Index) Push(ops ...IndexPushOption) error {
	var imageIndex v1.ImageIndex = i.ImageIndex
	var pushOps = &PushOptions{}

	if len(i.removedManifests) != 0 || len(i.annotate.instance) != 0 {
		return errors.New("index must need to be saved before pushing")
	}

	for _, op := range ops {
		err := op(pushOps)
		if err != nil {
			return err
		}
	}

	ref, err := name.ParseReference(i.Options.Reponame, name.WeakValidation, name.Insecure)
	if err != nil {
		return err
	}

	if pushOps.format != "" {
		mfest, err := i.IndexManifest()
		if err != nil {
			return err
		}

		if mfest == nil {
			return errors.New("index manifest is undefined")
		}

		if pushOps.format != mfest.MediaType {
			imageIndex = mutate.IndexMediaType(imageIndex, pushOps.format)
		}
	}

	err = remote.WriteIndex(ref, imageIndex, remote.WithAuthFromKeychain(i.Options.KeyChain), remote.WithTransport(getTransport(pushOps.insecure)))
	if err != nil {
		return err
	}

	if pushOps.purge {
		return i.Delete()
	}

	return nil
}

func(i *Index) Inspect() error {
	bytes, err := i.RawManifest()
	if err != nil {
		return err
	}

	if len(i.removedManifests) != 0 || len(i.annotate.instance) != 0 {
		return errors.New("index must need to be saved before inspecting")
	}

	return errors.New(string(bytes))
}

func(i *Index) Remove(digest name.Digest) error {
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

	i.removedManifests = append(i.removedManifests, hash)

	return nil
}

func(i *Index) Delete() error {
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
		return urls, errors.New("index manifest is undefined")
	}

	if mfest.Subject == nil {
		mfest.Subject = &v1.Descriptor{}
	}

	if len(mfest.Subject.URLs) == 0 {
		return urls, errors.New("urls is undefined")
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
		return urls, errors.New("urls is undefined")
	}

	return mfest.Subject.URLs, nil
}

func getConfigFile(img v1.Image) (config *v1.ConfigFile, err error) {
	config, err = img.ConfigFile()
	if err != nil {
		return
	}

	if config == nil {
		return config, errors.New("image config file is nil")
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
		return mfest, errors.New("index manifest is undefined")
	}

	return mfest, err
}

func getConfigFilePlatform(config v1.ConfigFile) (platform *v1.Platform, err error) {
	platform = config.Platform()
	if platform == nil {
		return platform, errors.New("platform is undefined")
	}
	return 
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