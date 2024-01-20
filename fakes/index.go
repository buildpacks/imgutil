package fakes

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
)

func NewIndex(format types.MediaType, byteSize, layers, count int64, ops ...Option) (*Index, error) {
	idx, err := ImageIndex(byteSize, layers, count, ops...)
	if err != nil {
		return nil, err
	}

	return &Index{
		ImageIndex: idx,
		format:     format,
		byteSize:   byteSize,
		layers:     layers,
		count:      count,
		ops:        ops,
	}, nil
}

type Index struct {
	os, arch, variant, osVersion map[v1.Hash]string
	features, osFeatures, urls   map[v1.Hash][]string
	annotations                  map[v1.Hash]map[string]string
	format                       types.MediaType
	byteSize, layers, count      int64
	ops                          []Option
	isDeleted                    bool
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

	i.urls[hash] = urls
	return nil
}

func (i *Index) Add(ref name.Reference, ops ...IndexAddOption) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	hash, err := v1.NewHash(ref.Identifier())
	if err != nil {
		return err
	}

	addOps := &IndexAddOptions{}
	for _, op := range ops {
		err := op(addOps)
		if err != nil {
			return err
		}
	}

	if idx, ok := i.ImageIndex.(*randomIndex); ok {
		if i.format == types.DockerManifestList {
			index, err := idx.AddImage(hash, types.DockerManifestSchema2, i.byteSize, i.layers, i.count, i.ops...)
			if err != nil {
				return err
			}
			i.ImageIndex = index
			return nil
		}
		index, err := idx.AddImage(hash, types.OCIManifestSchema1, i.byteSize, i.layers, i.count, i.ops...)
		if err != nil {
			return err
		}
		i.ImageIndex = index
		return nil
	}

	return errors.New("index is not random index")
}

func (i *Index) Save() error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	return nil
}

func (i *Index) Push(ops ...IndexPushOption) error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	return nil
}

func (i *Index) Inspect() error {
	if i.isDeleted {
		return errors.New("index doesn't exists")
	}

	bytes, err := i.ImageIndex.RawManifest()
	if err != nil {
		return err
	}

	return errors.New(string(bytes))
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
	return nil
}

type randomIndex struct {
	images   map[v1.Hash]v1.Image
	manifest *v1.IndexManifest
}

// Index returns a pseudo-randomly generated ImageIndex with count images, each
// having the given number of layers of size byteSize.
func ImageIndex(byteSize, layers, count int64, options ...Option) (v1.ImageIndex, error) {
	manifest := v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
	}

	images := make(map[v1.Hash]v1.Image)
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

	return &randomIndex{
		images:   images,
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
	// This is a single level index (for now?).
	return nil, fmt.Errorf("image not found: %v", h)
}

func (i *randomIndex) AddImage(hash v1.Hash, format types.MediaType, byteSize, layers, count int64, options ...Option) (v1.ImageIndex, error) {
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
