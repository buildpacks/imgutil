package fakes

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

func NewIndex(format types.MediaType, byteSize, layers, count int64, desc v1.Descriptor, ops ...Option) (*Index, error) {
	var (
		annotate = make(map[v1.Hash]v1.Descriptor, 0)
		images   = make(map[v1.Hash]v1.Image, 0)
	)

	idx, err := ImageIndex(byteSize, layers, count, desc, ops...)
	if err != nil {
		return nil, err
	}

	mfest, err := idx.IndexManifest()
	if err != nil {
		return nil, err
	}

	if mfest == nil {
		mfest = &v1.IndexManifest{}
	}

	for _, m := range mfest.Manifests {
		img, err := idx.Image(m.Digest)
		if err != nil {
			return nil, err
		}

		images[m.Digest] = img

		config, err := img.ConfigFile()
		if err != nil {
			return nil, err
		}

		if config == nil {
			config = &v1.ConfigFile{}
		}

		imgMfest, err := img.Manifest()
		if err != nil {
			return nil, err
		}

		if imgMfest == nil {
			imgMfest = &v1.Manifest{}
		}

		platform := imgMfest.Config.Platform

		if platform == nil && imgMfest.Subject != nil && imgMfest.Subject.Platform != nil {
			platform = imgMfest.Subject.Platform
		}

		if platform == nil {
			platform = &v1.Platform{}
		}

		annotate[m.Digest] = v1.Descriptor{
			Platform: &v1.Platform{
				OS:           config.OS,
				Architecture: config.Architecture,
				Variant:      config.Variant,
				OSVersion:    config.OSVersion,
				Features:     platform.Features,
				OSFeatures:   config.OSFeatures,
			},
			Annotations: imgMfest.Annotations,
			URLs:        imgMfest.Config.URLs,
		}
	}

	return &Index{
		ImageIndex: idx,
		format:     format,
		byteSize:   byteSize,
		layers:     layers,
		count:      count,
		ops:        ops,
		Annotate:   annotate,
		images:     images,
	}, nil
}

func computeIndex(idx *Index) error {
	mfest, err := idx.IndexManifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		mfest = &v1.IndexManifest{}
	}

	for _, m := range mfest.Manifests {
		img, err := idx.Image(m.Digest)
		if err != nil {
			return err
		}

		idx.images[m.Digest] = img

		config, err := img.ConfigFile()
		if err != nil {
			return err
		}

		if config == nil {
			config = &v1.ConfigFile{}
		}

		imgMfest, err := img.Manifest()
		if err != nil {
			return err
		}

		if imgMfest == nil {
			imgMfest = &v1.Manifest{}
		}

		platform := imgMfest.Config.Platform

		if platform == nil && imgMfest.Subject != nil && imgMfest.Subject.Platform != nil {
			platform = imgMfest.Subject.Platform
		}

		if platform == nil {
			platform = &v1.Platform{}
		}

		idx.Annotate[m.Digest] = v1.Descriptor{
			Platform: &v1.Platform{
				OS:           config.OS,
				Architecture: config.Architecture,
				OSVersion:    config.OSVersion,
				OSFeatures:   config.OSFeatures,
				Variant:      config.Variant,
				Features:     platform.Features,
			},
			Annotations: imgMfest.Annotations,
			URLs:        imgMfest.Config.URLs,
		}
	}
	return nil
}

var _ imgutil.ImageIndex = (*Index)(nil)

type Index struct {
	Annotate                        map[v1.Hash]v1.Descriptor
	format                          types.MediaType
	byteSize, layers, count         int64
	ops                             []Option
	isDeleted, shouldSave, AddIndex bool
	images                          map[v1.Hash]v1.Image
	v1.ImageIndex
}

func (i *Index) compute() {
	for h, v := range i.Annotate {
		i.ImageIndex = mutate.AppendManifests(i.ImageIndex, mutate.IndexAddendum{
			Add:        i.images[h],
			Descriptor: v,
		})
	}
}

func (i *Index) OS(digest name.Digest) (os string, err error) {
	i.compute()
	if i.isDeleted {
		return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.Platform.OS, nil
	}

	return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) Architecture(digest name.Digest) (arch string, err error) {
	i.compute()
	if i.isDeleted {
		return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.Platform.Architecture, nil
	}

	return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) Variant(digest name.Digest) (osVariant string, err error) {
	i.compute()
	if i.isDeleted {
		return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.Platform.Variant, nil
	}

	return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) OSVersion(digest name.Digest) (osVersion string, err error) {
	i.compute()
	if i.isDeleted {
		return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.Platform.OSVersion, nil
	}

	return "", imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) Features(digest name.Digest) (features []string, err error) {
	i.compute()
	if i.isDeleted {
		return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.Platform.Features, nil
	}

	return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	i.compute()
	if i.isDeleted {
		return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.Platform.OSFeatures, nil
	}

	return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	i.compute()
	if i.isDeleted {
		return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	if i.format == types.DockerManifestList {
		return nil, nil
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.Annotations, nil
	}

	return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) URLs(digest name.Digest) (urls []string, err error) {
	i.compute()
	if i.isDeleted {
		return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if desc, ok := i.Annotate[hash]; ok {
		return desc.URLs, nil
	}

	return nil, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
}

func (i *Index) SetOS(digest name.Digest, os string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.Platform.OS = os
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) SetArchitecture(digest name.Digest, arch string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	platform := desc.Platform
	platform.Architecture = arch
	desc.Platform = platform
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) SetVariant(digest name.Digest, osVariant string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	platform := desc.Platform
	platform.Variant = osVariant
	desc.Platform = platform
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) SetOSVersion(digest name.Digest, osVersion string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	platform := desc.Platform
	platform.OSVersion = osVersion
	desc.Platform = platform
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) SetFeatures(digest name.Digest, features []string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}
	platform := desc.Platform
	platform.Features = append(desc.Platform.Features, features...)
	desc.Platform = platform
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}
	platform := desc.Platform
	platform.OSFeatures = append(desc.Platform.OSFeatures, osFeatures...)
	desc.Platform = platform
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	if len(desc.Annotations) == 0 {
		desc.Annotations = make(map[string]string, 0)
	}

	for k, v := range annotations {
		desc.Annotations[k] = v
	}
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) SetURLs(digest name.Digest, urls []string) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	if _, err := i.OS(digest); err != nil {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = true
	desc := i.Annotate[hash]
	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	desc.URLs = append(desc.URLs, urls...)
	i.Annotate[hash] = desc
	return nil
}

func (i *Index) Add(ref name.Reference, ops ...imgutil.IndexAddOption) error {
	hash, err := v1.NewHash(ref.Identifier())
	if err != nil {
		length := 4
		b := make([]byte, length)
		hash, _, err = v1.SHA256(strings.NewReader(string(b)))
		if err != nil {
			return err
		}
	}

	addOps := &imgutil.AddOptions{}
	for _, op := range ops {
		op(addOps)
	}

	desc := func(format types.MediaType) v1.Descriptor {
		return v1.Descriptor{
			Digest:      hash,
			MediaType:   format,
			Annotations: addOps.Annotations,
			Platform: &v1.Platform{
				OS:           addOps.OS,
				Architecture: addOps.Arch,
				OSVersion:    addOps.OSVersion,
				Variant:      addOps.Variant,
				Features:     addOps.Features,
				OSFeatures:   addOps.OSFeatures,
			},
		}
	}

	if idx, ok := i.ImageIndex.(*randomIndex); ok {
		if i.AddIndex {
			if i.format == types.DockerManifestList {
				imgs, err := idx.addIndex(hash, types.DockerManifestSchema2, i.byteSize, i.layers, i.count, *addOps)
				if err != nil {
					return err
				}

				for _, img := range imgs {
					err := i.addImage(img, desc(types.DockerManifestSchema2))
					if err != nil {
						return err
					}
				}
			}
			imgs, err := idx.addIndex(hash, types.OCIManifestSchema1, i.byteSize, i.layers, i.count, *addOps)
			if err != nil {
				return err
			}

			for _, img := range imgs {
				err := i.addImage(img, desc(types.OCIManifestSchema1))
				if err != nil {
					return err
				}
			}
		}
		if i.format == types.DockerManifestList {
			img, err := idx.addImage(hash, types.DockerManifestSchema2, i.byteSize, i.layers, i.count, *addOps)
			if err != nil {
				return err
			}

			return i.addImage(img, desc(types.DockerManifestSchema2))
		}
		img, err := idx.addImage(hash, types.OCIManifestSchema1, i.byteSize, i.layers, i.count, *addOps)
		if err != nil {
			return err
		}

		return i.addImage(img, desc(types.OCIManifestSchema1))
	}

	return errors.New("index is not random index")
}

func (i *Index) addImage(image v1.Image, desc v1.Descriptor) error {
	i.shouldSave = true
	if err := satisifyPlatform(image, &desc); err != nil {
		return err
	}

	if config, err := configFromDesc(image, desc); err == nil {
		image, err = mutate.ConfigFile(image, config)
		if err != nil {
			return err
		}
	}

	image = mutate.Subject(image, desc).(v1.Image)
	i.ImageIndex = mutate.AppendManifests(i.ImageIndex, mutate.IndexAddendum{
		Add:        image,
		Descriptor: desc,
	})
	return computeIndex(i)
}

func configFromDesc(image v1.Image, desc v1.Descriptor) (*v1.ConfigFile, error) {
	config, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	if config == nil {
		return nil, imgutil.ErrConfigFileUndefined
	}

	if desc.Platform == nil {
		desc.Platform = &v1.Platform{}
	}

	switch p := desc.Platform; {
	case p.OS != "":
		config.OS = p.OS
		fallthrough
	case p.Architecture != "":
		config.Architecture = p.Architecture
		fallthrough
	case p.Variant != "":
		config.Variant = p.Variant
		fallthrough
	case p.OSVersion != "":
		config.OSVersion = p.OSVersion
		fallthrough
	case len(p.Features) != 0:
		plat := config.Platform()
		if plat == nil {
			plat = &v1.Platform{}
		}

		plat.Features = append(plat.Features, p.Features...)
		fallthrough
	case len(p.OSFeatures) != 0:
		config.OSFeatures = append(config.OSFeatures, p.OSFeatures...)
	}

	return config, nil
}

func satisifyPlatform(image v1.Image, desc *v1.Descriptor) error {
	config, err := image.ConfigFile()
	if err != nil {
		return err
	}

	if config == nil {
		return imgutil.ErrConfigFileUndefined
	}

	mfest, err := image.Manifest()
	if err != nil {
		return err
	}

	if mfest == nil {
		return imgutil.ErrManifestUndefined
	}

	features := make([]string, 0)
	if p := config.Platform(); p != nil {
		features = p.Features
	}

	platform := &v1.Platform{
		OS:           config.OS,
		Architecture: config.Architecture,
		Variant:      config.Variant,
		OSVersion:    config.OSVersion,
		Features:     features,
		OSFeatures:   config.OSFeatures,
	}

	if p := desc.Platform; !p.Equals(*platform) {
		switch {
		case p.OS != "":
			platform.OS = p.OS
			fallthrough
		case p.Architecture != "":
			platform.Architecture = p.Architecture
			fallthrough
		case p.Variant != "":
			platform.Variant = p.Variant
			fallthrough
		case p.OSVersion != "":
			platform.OSVersion = p.OSVersion
			fallthrough
		case len(p.Features) != 0:
			platform.Features = append(platform.Features, p.Features...)
			fallthrough
		case len(p.OSFeatures) != 0:
			platform.OSFeatures = append(platform.OSFeatures, p.OSFeatures...)
		}
	}

	annos := make(map[string]string)
	if len(mfest.Annotations) != 0 {
		annos = mfest.Annotations
	}

	if len(desc.Annotations) != 0 {
		for k, v := range mfest.Annotations {
			annos[k] = v
		}
	}

	desc = &v1.Descriptor{
		Annotations: annos,
		Platform:    platform,
	}
	return nil
}

func (i *Index) Save() error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.shouldSave = false
	return nil
}

func (i *Index) Push(ops ...imgutil.IndexPushOption) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	if i.shouldSave {
		return errors.New("index should need to be saved")
	}

	return nil
}

func (i *Index) Inspect() (mfestStr string, err error) {
	i.compute()
	if i.isDeleted {
		return mfestStr, imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	if i.shouldSave {
		return mfestStr, errors.New("index should need to be saved")
	}

	mfest, err := i.ImageIndex.IndexManifest()
	if err != nil {
		return mfestStr, err
	}

	if mfest == nil {
		return mfestStr, imgutil.ErrManifestUndefined
	}

	mfestBytes, err := json.MarshalIndent(mfest, "", "	")
	if err != nil {
		return mfestStr, err
	}

	return string(mfestBytes), nil
}

func (i *Index) Remove(digest name.Reference) error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	delete(i.images, hash)
	delete(i.Annotate, hash)
	return nil
}

func (i *Index) Delete() error {
	if i.isDeleted {
		return imgutil.ErrNoImageOrIndexFoundWithGivenDigest
	}

	i.isDeleted = true
	i.shouldSave = false
	return nil
}

type randomIndex struct {
	images   map[v1.Hash]v1.Image
	indexes  map[v1.Hash]v1.ImageIndex
	manifest *v1.IndexManifest
}

// Index returns a pseudo-randomly generated ImageIndex with count images, each
// having the given number of layers of size byteSize.
func ImageIndex(byteSize, layers, count int64, desc v1.Descriptor, options ...Option) (v1.ImageIndex, error) {
	manifest := v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
		Subject:       &desc,
	}

	images := make(map[v1.Hash]v1.Image)

	indexes := make(map[v1.Hash]v1.ImageIndex)
	withouIndex := WithIndex(false)
	o := getOptions(options)

	if o.withIndex {
		withouIndex(o)
		options = append(options, WithSource(o.source), WithIndex(o.withIndex))
		for i := int64(0); i < count; i++ {
			idx, err := ImageIndex(byteSize, layers, count, desc, options...)
			if err != nil {
				return nil, err
			}

			rawManifest, err := idx.RawManifest()
			if err != nil {
				return nil, err
			}
			digest, size, err := v1.SHA256(bytes.NewReader(rawManifest))
			if err != nil {
				return nil, err
			}
			mediaType, err := idx.MediaType()
			if err != nil {
				return nil, err
			}

			manifest.Manifests = append(manifest.Manifests, v1.Descriptor{
				Digest:    digest,
				Size:      size,
				MediaType: mediaType,
			})

			indexes[digest] = idx
		}
	} else {
		for i := int64(0); i < count; i++ {
			img, err := V1Image(byteSize, layers, options...)
			if err != nil {
				return nil, err
			}

			rawManifest, err := img.RawManifest()
			if err != nil {
				return nil, err
			}
			digest, size, err := v1.SHA256(bytes.NewReader(rawManifest))
			if err != nil {
				return nil, err
			}
			mediaType, err := img.MediaType()
			if err != nil {
				return nil, err
			}

			manifest.Manifests = append(manifest.Manifests, v1.Descriptor{
				Digest:    digest,
				Size:      size,
				MediaType: mediaType,
			})

			images[digest] = img
		}
	}

	return &randomIndex{
		images:   images,
		indexes:  indexes,
		manifest: &manifest,
	}, nil
}

func (i *randomIndex) MediaType() (types.MediaType, error) {
	return i.manifest.MediaType, nil
}

func (i *randomIndex) Digest() (v1.Hash, error) {
	return partial.Digest(i)
}

func (i *randomIndex) Size() (int64, error) {
	return partial.Size(i)
}

func (i *randomIndex) IndexManifest() (*v1.IndexManifest, error) {
	return i.manifest, nil
}

func (i *randomIndex) RawManifest() ([]byte, error) {
	m, err := i.IndexManifest()
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func (i *randomIndex) Image(h v1.Hash) (v1.Image, error) {
	if img, ok := i.images[h]; ok {
		return img, nil
	}

	return nil, fmt.Errorf("image not found: %v", h)
}

func (i *randomIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	if idx, ok := i.indexes[h]; ok {
		return idx, nil
	}

	return nil, fmt.Errorf("image not found: %v", h)
}

func (i *randomIndex) addImage(hash v1.Hash, format types.MediaType, byteSize, layers, count int64, options imgutil.AddOptions) (v1.Image, error) {
	img, err := V1Image(byteSize, layers)
	if err != nil {
		return img, err
	}

	rawManifest, err := img.RawManifest()
	if err != nil {
		return img, err
	}
	_, size, err := v1.SHA256(bytes.NewReader(rawManifest))
	if err != nil {
		return img, err
	}

	i.manifest.Manifests = append(i.manifest.Manifests, v1.Descriptor{
		Digest:      hash,
		Size:        size,
		MediaType:   format,
		Annotations: options.Annotations,
		Platform: &v1.Platform{
			OS:           options.OS,
			Architecture: options.Arch,
			Variant:      options.Variant,
			OSVersion:    options.OSVersion,
			Features:     options.Features,
			OSFeatures:   options.OSFeatures,
		},
	})

	i.images[hash] = img

	return img, nil
}

func randStr() (string, error) {
	length := 10 // adjust the length as needed

	b := make([]byte, length)
	_, err := rand.Read(b) // read random bytes
	if err != nil {
		fmt.Println("Error generating random bytes:", err)
		return "", err
	}

	return base64.URLEncoding.EncodeToString(b)[:length], nil
}

func (i *randomIndex) addIndex(hash v1.Hash, format types.MediaType, byteSize, layers, count int64, ops imgutil.AddOptions) ([]v1.Image, error) {
	switch {
	case ops.All:
		var images = make([]v1.Image, 0)
		for _, v := range AllPlatforms {
			str, err := randStr()
			if err != nil {
				return nil, err
			}

			d, _, err := v1.SHA256(bytes.NewReader([]byte(str)))
			if err != nil {
				return nil, err
			}

			img, err := i.addImage(d, format, byteSize, layers, 1, imgutil.AddOptions{
				OS:      v.OS,
				Arch:    v.Arch,
				Variant: v.Variant,
			})
			if err != nil {
				return nil, err
			}

			images = append(images, img)
		}

		return images, nil
	case ops.OS != "",
		ops.Arch != "",
		ops.Variant != "",
		ops.OSVersion != "",
		len(ops.Features) != 0,
		len(ops.OSFeatures) != 0,
		len(ops.Annotations) != 0:
		img, err := i.addImage(hash, format, byteSize, layers, 1, ops)
		return []v1.Image{img}, err
	default:
		img, err := i.addImage(hash, format, byteSize, layers, 1, imgutil.AddOptions{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		})
		return []v1.Image{img}, err
	}
}

type Platform struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Variant string `json:"variant,omitempty"` // Optional variant field
}

var AllPlatforms = map[string]Platform{
	"linux/amd64":     {OS: "linux", Arch: "amd64"},
	"linux/arm64":     {OS: "linux", Arch: "arm64"},
	"linux/386":       {OS: "linux", Arch: "386"},
	"linux/mips64":    {OS: "linux", Arch: "mips64"},
	"linux/mipsle":    {OS: "linux", Arch: "mipsle"},
	"linux/ppc64le":   {OS: "linux", Arch: "ppc64le"},
	"linux/s390x":     {OS: "linux", Arch: "s390x"},
	"darwin/amd64":    {OS: "darwin", Arch: "amd64"},
	"darwin/arm64":    {OS: "darwin", Arch: "arm64"},
	"windows/amd64":   {OS: "windows", Arch: "amd64"},
	"windows/386":     {OS: "windows", Arch: "386"},
	"freebsd/amd64":   {OS: "freebsd", Arch: "amd64"},
	"freebsd/386":     {OS: "freebsd", Arch: "386"},
	"netbsd/amd64":    {OS: "netbsd", Arch: "amd64"},
	"netbsd/386":      {OS: "netbsd", Arch: "386"},
	"openbsd/amd64":   {OS: "openbsd", Arch: "amd64"},
	"openbsd/386":     {OS: "openbsd", Arch: "386"},
	"dragonfly/amd64": {OS: "dragonfly", Arch: "amd64"},
	"dragonfly/386":   {OS: "dragonfly", Arch: "386"},
}
