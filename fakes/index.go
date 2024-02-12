package fakes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

func NewIndex(format types.MediaType, byteSize, layers, count int64, desc v1.Descriptor, ops ...Option) (*Index, error) {
	var (
		os          = make(map[v1.Hash]string, count)
		arch        = make(map[v1.Hash]string, count)
		variant     = make(map[v1.Hash]string, count)
		osVersion   = make(map[v1.Hash]string, count)
		features    = make(map[v1.Hash][]string, count)
		osFeatures  = make(map[v1.Hash][]string, count)
		urls        = make(map[v1.Hash][]string, count)
		annotations = make(map[v1.Hash]map[string]string, count)
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

		config, err := img.ConfigFile()
		if err != nil {
			return nil, err
		}

		if config == nil {
			config = &v1.ConfigFile{}
		}

		os[m.Digest] = config.OS
		arch[m.Digest] = config.Architecture
		variant[m.Digest] = config.Variant
		osVersion[m.Digest] = config.OSVersion
		osFeatures[m.Digest] = config.OSFeatures

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

		features[m.Digest] = platform.Features
		annotations[m.Digest] = imgMfest.Annotations
		urls[m.Digest] = imgMfest.Config.URLs
	}

	return &Index{
		ImageIndex:  idx,
		format:      format,
		byteSize:    byteSize,
		layers:      layers,
		count:       count,
		ops:         ops,
		os:          os,
		arch:        arch,
		variant:     variant,
		osVersion:   osVersion,
		features:    features,
		osFeatures:  osFeatures,
		urls:        urls,
		annotations: annotations,
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

		config, err := img.ConfigFile()
		if err != nil {
			return err
		}

		if config == nil {
			config = &v1.ConfigFile{}
		}

		idx.os[m.Digest] = config.OS
		idx.arch[m.Digest] = config.Architecture
		idx.variant[m.Digest] = config.Variant
		idx.osVersion[m.Digest] = config.OSVersion
		idx.osFeatures[m.Digest] = config.OSFeatures

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

		idx.features[m.Digest] = platform.Features
		idx.annotations[m.Digest] = imgMfest.Annotations
		idx.urls[m.Digest] = imgMfest.Config.URLs
	}
	return nil
}

var _ imgutil.ImageIndex = (*Index)(nil)

type Index struct {
	os, arch, variant, osVersion    map[v1.Hash]string
	features, osFeatures, urls      map[v1.Hash][]string
	annotations                     map[v1.Hash]map[string]string
	format                          types.MediaType
	byteSize, layers, count         int64
	ops                             []Option
	isDeleted, shouldSave, AddIndex bool
	v1.ImageIndex
}

func (i *Index) OS(digest name.Digest) (os string, err error) {
	if i.isDeleted {
		return "", errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.os[hash]; ok {
		return v, nil
	}

	return "", errors.New("no image/index found with the given digest")
}

func (i *Index) Architecture(digest name.Digest) (arch string, err error) {
	if i.isDeleted {
		return "", errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.arch[hash]; ok {
		return v, nil
	}

	return "", errors.New("no image/index found with the given digest")
}

func (i *Index) Variant(digest name.Digest) (osVariant string, err error) {
	if i.isDeleted {
		return "", errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.variant[hash]; ok {
		return v, nil
	}

	return "", errors.New("no image/index found with the given digest")
}

func (i *Index) OSVersion(digest name.Digest) (osVersion string, err error) {
	if i.isDeleted {
		return "", errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.osVersion[hash]; ok {
		return v, nil
	}

	return "", errors.New("no image/index found with the given digest")
}

func (i *Index) Features(digest name.Digest) (features []string, err error) {
	if i.isDeleted {
		return nil, errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.features[hash]; ok {
		return v, nil
	}

	return nil, errors.New("no image/index found with the given digest")
}

func (i *Index) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	if i.isDeleted {
		return nil, errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.osFeatures[hash]; ok {
		return v, nil
	}

	return nil, errors.New("no image/index found with the given digest")
}

func (i *Index) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	if i.isDeleted {
		return nil, errors.New("index doesn't exists")
	}

	if i.format == types.DockerManifestList {
		return nil, nil
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.annotations[hash]; ok {
		return v, nil
	}

	return nil, errors.New("no image/index found with the given digest")
}

func (i *Index) URLs(digest name.Digest) (urls []string, err error) {
	if i.isDeleted {
		return nil, errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return
	}

	if v, ok := i.urls[hash]; ok {
		return v, nil
	}

	return nil, errors.New("no image/index found with the given digest")
}

func (i *Index) SetOS(digest name.Digest, os string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.os[hash] = os
	return nil
}

func (i *Index) SetArchitecture(digest name.Digest, arch string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.arch[hash] = arch
	return nil
}

func (i *Index) SetVariant(digest name.Digest, osVariant string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.variant[hash] = osVariant
	return nil
}

func (i *Index) SetOSVersion(digest name.Digest, osVersion string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.osVersion[hash] = osVersion
	return nil
}

func (i *Index) SetFeatures(digest name.Digest, features []string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.features[hash] = features
	return nil
}

func (i *Index) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.osFeatures[hash] = osFeatures
	return nil
}

func (i *Index) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.annotations[hash] = annotations
	return nil
}

func (i *Index) SetURLs(digest name.Digest, urls []string) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	i.shouldSave = true
	i.urls[hash] = urls
	return nil
}

func (i *Index) Add(ref name.Reference, ops ...imgutil.IndexAddOption) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

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

	if idx, ok := i.ImageIndex.(*randomIndex); ok {
		if i.AddIndex {
			if i.format == types.DockerManifestList {
				index, err := idx.addIndex(hash, types.DockerManifestSchema2, i.byteSize, i.layers, i.count, i.ops...)
				if err != nil {
					return err
				}

				i.shouldSave = true
				i.ImageIndex = index
				return computeIndex(i)
			}
			index, err := idx.addIndex(hash, types.OCIManifestSchema1, i.byteSize, i.layers, i.count, i.ops...)
			if err != nil {
				return err
			}

			i.shouldSave = true
			i.ImageIndex = index
			return computeIndex(i)
		}
		if i.format == types.DockerManifestList {
			index, err := idx.addImage(hash, types.DockerManifestSchema2, i.byteSize, i.layers, i.count, i.ops...)
			if err != nil {
				return err
			}

			i.shouldSave = true
			i.ImageIndex = index
			return computeIndex(i)
		}
		index, err := idx.addImage(hash, types.OCIManifestSchema1, i.byteSize, i.layers, i.count, i.ops...)
		if err != nil {
			return err
		}

		i.shouldSave = true
		i.ImageIndex = index
		return computeIndex(i)
	}

	return errors.New("index is not random index")
}

func (i *Index) Save() error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	i.shouldSave = false
	return nil
}

func (i *Index) Push(ops ...imgutil.IndexPushOption) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	if i.shouldSave {
		return errors.New("index should need to be saved")
	}

	return nil
}

func (i *Index) Inspect() (mfestStr string, err error) {
	if i.isDeleted {
		return mfestStr, errors.New("index doesn't exists")
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

func (i *Index) Remove(digest name.Digest) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	delete(i.os, hash)
	delete(i.arch, hash)
	delete(i.variant, hash)
	delete(i.osVersion, hash)
	delete(i.features, hash)
	delete(i.osFeatures, hash)
	delete(i.annotations, hash)
	delete(i.urls, hash)
	return nil
}

func (i *Index) Delete() error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
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

func (i *randomIndex) addImage(hash v1.Hash, format types.MediaType, byteSize, layers, count int64, options ...Option) (v1.ImageIndex, error) {
	img, err := V1Image(byteSize, layers, options...)
	if err != nil {
		return nil, err
	}

	rawManifest, err := img.RawManifest()
	if err != nil {
		return nil, err
	}
	_, size, err := v1.SHA256(bytes.NewReader(rawManifest))
	if err != nil {
		return nil, err
	}

	i.manifest.Manifests = append(i.manifest.Manifests, v1.Descriptor{
		Digest:    hash,
		Size:      size,
		MediaType: format,
	})

	i.images[hash] = img

	return i, nil
}

func (i *randomIndex) addIndex(hash v1.Hash, format types.MediaType, byteSize, layers, count int64, options ...Option) (v1.ImageIndex, error) {
	idx, err := ImageIndex(byteSize, layers, count, v1.Descriptor{}, options...)
	if err != nil {
		return nil, err
	}

	rawManifest, err := idx.RawManifest()
	if err != nil {
		return nil, err
	}
	_, size, err := v1.SHA256(bytes.NewReader(rawManifest))
	if err != nil {
		return nil, err
	}

	i.manifest.Manifests = append(i.manifest.Manifests, v1.Descriptor{
		Digest:    hash,
		Size:      size,
		MediaType: format,
	})

	i.indexes[hash] = idx

	return i, nil
}
