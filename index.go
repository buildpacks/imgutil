package imgutil

import (
	"bytes"
	"encoding/json"
	"reflect"
	"runtime"

	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type Index interface {
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
	Push(ops ...IndexAddOption) error
	Inspect() error
	Remove(digest name.Digest) error
	Delete() error
}

type ImageIndex struct {
	Handler Index
}

type ManifestAction int
type NewManifest map[v1.Hash][]byte
type InstanceMap map[v1.Hash][]instance
type IndexMap map[v1.Hash][]v1.Manifest
type IndexOption func(*IndexStruct) error
type IndexAddOption func(*IndexAddOptions)

const (
	ADD ManifestAction = iota
	REPLACE
	DELETE
)

type ImageIndexHandler struct {
	IndexStruct
}

type ManifestHandler struct {
	IndexStruct
}

var _ Index = (*ManifestHandler)(nil)

type instance struct {
	action     ManifestAction
	options    []layout.Option
	hash       v1.Hash
	isIndex    bool
	image      *v1.Image
	index      *v1.ImageIndex
	descriptor *v1.Descriptor
}

type IndexAddOptions struct {
	all, purge                   bool
	os, arch, variant, osVersion string
	features, osFeatures         []string
	annotations                  map[string]string
	format                       MediaTypes
}

func NewIndex(name string, manifestOnly bool, ops ...IndexOption) (index *ImageIndex, err error) {
	idxOps := &IndexStruct{}
	for _, op := range ops {
		if err := op(idxOps); err != nil {
			return index, err
		}
	}

	idxRootPath := filepath.Join(idxOps.XdgRuntimePath(), name)

	_, err = layout.FromPath(idxRootPath)
	if err != nil {
		return index, fmt.Errorf("imageIndex with the given name doesn't exists")
	}

	idxMapPath := filepath.Join(idxRootPath, "index.map.json")
	if _, err = os.Stat(idxMapPath); err == nil {
		file, err := os.Open(idxMapPath)
		if err == nil {
			var idxMap = &IndexMap{}
			err = json.NewDecoder(file).Decode(idxMap)
			if err != nil {
				return index, err
			}

			idxOps.IndexMap(idxMap)
		}
	}

	if manifestOnly {
		index = &ImageIndex{
			Handler: &ManifestHandler{
				IndexStruct: *idxOps,
			},
		}
	}

	return index, nil
}

func (o *IndexAddOptions) LayoutOptions() (ops []layout.Option) {
	platform := v1.Platform{
		Architecture: o.arch,
		OS:           o.os,
		OSVersion:    o.osVersion,
		Features:     o.features,
		Variant:      o.variant,
		OSFeatures:   o.osFeatures,
	}

	switch {
	case len(o.annotations) != 0:
		ops = append(ops, layout.WithAnnotations(o.annotations))
	case o.arch != "":
	case len(o.features) != 0:
	case o.os != "":
	case len(o.osFeatures) != 0:
	case o.osVersion != "":
	case o.variant != "":
		ops = append(ops, layout.WithPlatform(platform))
	}

	return ops
}

func WithFormat(format MediaTypes) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.format = format
	}
}

func WithAll(all bool) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.all = all
	}
}

func WithOS(os string) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.os = os
	}
}

func WithArch(arch string) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.arch = arch
	}
}

func WithVariant(variant string) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.variant = variant
	}
}

func WithOSVersion(version string) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.osVersion = version
	}
}

func WithFeatures(features []string) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.features = features
	}
}

func WithAnnotaions(annotations map[string]string) IndexAddOption {
	return func(o *IndexAddOptions) {
		o.annotations = annotations
	}
}

func (m *IndexMap) AddIndex(index *v1.IndexManifest, hash v1.Hash, repoName string, keys authn.Keychain) (manifest []*v1.Manifest, err error) {
	manifests, ok := (*m)[hash]

	for _, descManifest := range index.Manifests {
		manifestBytes, err := json.MarshalIndent(descManifest, "", "   ")
		if err != nil {
			return manifest, err
		}

		if descManifest.MediaType.IsImage() {
			mfest, err := v1.ParseManifest(bytes.NewReader(manifestBytes))
			if err != nil {
				return manifest, err
			}

			manifest = append(manifest, mfest)

			switch ok {
			case true:
				manifests = append(manifests, *mfest)
			case false:
				manifests = []v1.Manifest{*mfest}
			}
		}

		if descManifest.MediaType.IsIndex() {
			mfest, err := v1.ParseIndexManifest(bytes.NewReader(manifestBytes))
			if err != nil {
				return manifest, err
			}
			m.AddIndex(mfest, hash, repoName, keys)
			m.AddIndex(mfest, mfest.Subject.Digest, repoName, keys)
		}
	}

	return manifest, err
}

func (m InstanceMap) get(hash v1.Hash) []instance {
	return m[hash]
}

func (m InstanceMap) Add(hash v1.Hash, instances []instance) {
	i, ok := m[hash]
	if !ok {
		m[hash] = instances
	} else {
		m[hash] = append(i, instances...)
	}
}

func (m *InstanceMap) AddDescriptor(desc *v1.Descriptor, ops ...layout.Option) error {
	hash := desc.Digest
	m.Add(hash, []instance{
		{
			action:     ADD,
			options:    ops,
			isIndex:    desc.MediaType.IsIndex(),
			hash:       hash,
			descriptor: desc,
		},
	})

	return nil
}

func (m *InstanceMap) AddImage(image *v1.Image, ops ...layout.Option) error {
	hash, err := (*image).Digest()
	if err != nil {
		return err
	}

	m.Add(hash, []instance{
		{
			action:  ADD,
			options: ops,
			image:   image,
			isIndex: false,
			hash:    hash,
		},
	})

	return err
}

func (m *InstanceMap) AddIndex(index *v1.ImageIndex, ops ...layout.Option) error {
	hash, err := (*index).Digest()
	if err != nil {
		return err
	}

	m.Add(hash, []instance{
		{
			action:  ADD,
			options: ops,
			index:   index,
			isIndex: true,
			hash:    hash,
		},
	})

	return nil
}

func (m *InstanceMap) Replace(hash v1.Hash, isIndex bool, ops ...layout.Option) {
	m.Add(hash, []instance{
		{
			action:  REPLACE,
			options: ops,
			isIndex: isIndex,
			hash:    hash,
		},
	})
}

func (m *InstanceMap) Remove(hash v1.Hash, isIndex bool, ops ...layout.Option) {
	m.Add(hash, []instance{
		{
			action:  DELETE,
			options: ops,
			isIndex: isIndex,
			hash:    hash,
		},
	})
}

func (m *NewManifest) GetRaw(hash v1.Hash) (bytes []byte, ok bool) {
	bytes, ok = (*m)[hash]
	return
}

func (m *NewManifest) Manifest(hash v1.Hash) (manifest *v1.Manifest, err error) {
	instance, ok := (*m)[hash]
	if !ok {
		return manifest, fmt.Errorf("no Image found with the given Hash: %s", hash.String())
	}

	err = json.Unmarshal(instance, manifest)
	if !manifest.MediaType.IsImage() {
		return manifest, fmt.Errorf("error validating Image Manifest")
	}

	return
}

func (m *NewManifest) IndexManifest(hash v1.Hash) (manifest *v1.IndexManifest, err error) {
	instance, ok := (*m)[hash]
	if !ok {
		return manifest, fmt.Errorf("no Image found with the given Hash: %s", hash.String())
	}

	err = json.Unmarshal(instance, manifest)
	if !manifest.MediaType.IsIndex() {
		return manifest, fmt.Errorf("error validating Index Manifest")
	}

	return
}

func (m *NewManifest) Set(hash v1.Hash, manifestBytes []byte) {
	(*m)[hash] = manifestBytes
}

func (m *NewManifest) Delete(hash v1.Hash) {
	_, ok := (*m)[hash]
	if !ok {
		return
	}

	delete(*m, hash)
}

type IndexStruct struct {
	keychain            authn.Keychain
	repoName            string
	index               *v1.ImageIndex
	requestedMediaTypes MediaTypes
	instance            *InstanceMap
	newManifest         *NewManifest
	indexMap            *IndexMap
	xdgRuntimePath      string
	ref                 name.Reference
	insecure            bool
}

func (i *IndexStruct) KeyChain() authn.Keychain {
	return i.keychain
}

func (i *IndexStruct) RepoName() string {
	return i.repoName
}

func (i *IndexStruct) XdgRuntimePath() string {
	return i.xdgRuntimePath
}

func (i *IndexStruct) Insecure() bool {
	return i.insecure
}

func (i *IndexStruct) IndexMap(indexMap *IndexMap) {
	i.indexMap = indexMap
}

func WithIndex(idx *v1.ImageIndex) IndexOption {
	return func(i *IndexStruct) error {
		i.index = idx
		return nil
	}
}

func WithKeyChain(keychain authn.Keychain) IndexOption {
	return func(i *IndexStruct) error {
		i.keychain = keychain
		return nil
	}
}

func WithRepoName(repoName string) IndexOption {
	return func(i *IndexStruct) error {
		i.repoName = repoName
		ref, err := name.ParseReference(repoName, name.WeakValidation)
		if err != nil {
			return err
		}

		i.ref = ref
		return nil
	}
}

func WithMediaTypes(mediaType MediaTypes) IndexOption {
	return func(i *IndexStruct) error {
		i.requestedMediaTypes = mediaType
		return nil
	}
}

func WithXDGRuntimePath(path string) IndexOption {
	return func(i *IndexStruct) error {
		i.xdgRuntimePath = path
		return nil
	}
}

func WithInsecure(insecure bool) IndexOption {
	return func(i *IndexStruct) error {
		i.insecure = insecure
		return nil
	}
}

func (i *ImageIndex) OS(digest name.Digest) (os string, err error) {
	return i.Handler.OS(digest)
}

func (i *ImageIndexHandler) OS(digest name.Digest) (os string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return os, err
	}

	manifest, err := i.newManifest.IndexManifest(hash)
	if err != nil {
		manifest, err := i.newManifest.Manifest(hash)
		if err != nil {
			return os, err
		}

		os = manifest.Config.Platform.OS

		if os == "" {
			return osFromPath(i.repoName, i.xdgRuntimePath, digestStr)
		}

		return os, err
	}

	os = manifest.Subject.Platform.OS

	if os == "" {
		return osFromPath(i.repoName, i.xdgRuntimePath, digestStr)
	}

	return os, err
}

func (i *ManifestHandler) OS(digest name.Digest) (os string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.Platform.OS, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.Platform.OS, err
	}

	return osFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func osFromPath(repoName, xdgRuntimePath, digestStr string) (os string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return os, err
		}

		config, err := img.ConfigFile()
		if err != nil || config == nil {
			return os, err
		}

		return config.OS, nil
	}

	return idx.Subject.Platform.OS, nil
}

func (i *ImageIndex) Architecture(digest name.Digest) (arch string, err error) {
	return i.Handler.Architecture(digest)
}

func (i *ManifestHandler) Architecture(digest name.Digest) (arch string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.Platform.Architecture, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.Platform.Architecture, err
	}

	return archFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func archFromPath(repoName, xdgRuntimePath, digestStr string) (arch string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return arch, err
		}

		config, err := img.ConfigFile()
		if err != nil || config == nil {
			return arch, err
		}

		return config.Architecture, nil
	}

	return idx.Subject.Platform.Architecture, nil
}

func (i *ImageIndex) Variant(digest name.Digest) (osVariant string, err error) {
	return i.Handler.Variant(digest)
}

func (i *ManifestHandler) Variant(digest name.Digest) (osVariant string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.Platform.Variant, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.Platform.Variant, err
	}

	return osVariantFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func osVariantFromPath(repoName, xdgRuntimePath, digestStr string) (osVariant string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return osVariant, err
		}

		config, err := img.ConfigFile()
		if err != nil || config == nil {
			return osVariant, err
		}

		return config.Variant, nil
	}

	return idx.Subject.Platform.Variant, nil
}

func (i *ImageIndex) OSVersion(digest name.Digest) (osVersion string, err error) {
	return i.Handler.OSVersion(digest)
}

func (i *ManifestHandler) OSVersion(digest name.Digest) (osVersion string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.Platform.OSVersion, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.Platform.OSVersion, err
	}

	return osVersionFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func osVersionFromPath(repoName, xdgRuntimePath, digestStr string) (osVersion string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return osVersion, err
		}

		config, err := img.ConfigFile()
		if err != nil || config == nil {
			return osVersion, err
		}

		return config.OSVersion, nil
	}

	return idx.Subject.Platform.OSVersion, nil
}

func (i *ImageIndex) Features(digest name.Digest) (features []string, err error) {
	return i.Handler.Features(digest)
}

func (i *ManifestHandler) Features(digest name.Digest) (features []string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.Platform.Features, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.Platform.Features, err
	}

	return featuresFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func featuresFromPath(repoName, xdgRuntimePath, digestStr string) (features []string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return features, err
		}

		config, err := img.ConfigFile()
		if err != nil || config == nil {
			return features, err
		}

		return config.Platform().Features, nil
	}

	return idx.Subject.Platform.Features, nil
}

func (i *ImageIndex) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	return i.Handler.OSFeatures(digest)
}

func (i *ManifestHandler) OSFeatures(digest name.Digest) (osFeatures []string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.Platform.OSFeatures, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.Platform.OSFeatures, err
	}

	return osFeaturesFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func osFeaturesFromPath(repoName, xdgRuntimePath, digestStr string) (osFeatures []string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return osFeatures, err
		}

		config, err := img.ConfigFile()
		if err != nil || config == nil {
			return osFeatures, err
		}

		return config.Platform().OSFeatures, nil
	}

	return idx.Subject.Platform.OSFeatures, nil
}

func (i *ImageIndex) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	return i.Handler.Annotations(digest)
}

func (i *ManifestHandler) Annotations(digest name.Digest) (annotations map[string]string, err error) {
	if i.requestedMediaTypes == DockerTypes {
		return
	}

	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.Annotations, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.Annotations, err
	}

	return annotationsFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func annotationsFromPath(repoName, xdgRuntimePath, digestStr string) (annotations map[string]string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return annotations, err
		}

		manifest, err := img.Manifest()
		if err != nil || manifest == nil {
			return annotations, err
		}

		return manifest.Annotations, nil
	}

	return idx.Annotations, nil
}

func (i *ImageIndex) URLs(digest name.Digest) (urls []string, err error) {
	return i.Handler.URLs(digest)
}

func (i *ManifestHandler) URLs(digest name.Digest) (urls []string, err error) {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return
	}

	imgIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		return imgIdx.Subject.URLs, err
	}

	manifest, err := i.newManifest.Manifest(hash)
	if err == nil {
		return manifest.Subject.URLs, err
	}

	return urlsFromPath(i.repoName, i.xdgRuntimePath, digestStr)
}

func urlsFromPath(repoName, xdgRuntimePath, digestStr string) (urls []string, err error) {
	idx, err := idxFromRepoName(repoName, xdgRuntimePath)
	if err != nil {
		img, err := imgFromRepoName(repoName, digestStr, xdgRuntimePath)
		if err != nil {
			return urls, err
		}

		manifest, err := img.Manifest()
		if err != nil || manifest == nil {
			return urls, err
		}

		urls = manifest.Config.URLs
		if len(urls) == 0 {
			urls = manifest.Subject.URLs
		}

		return urls, nil
	}

	return idx.Subject.URLs, nil
}

func imgFromRepoName(repoName, hashString, xdgRuntimePath string) (image v1.Image, err error) {
	idxPath, err := layoutPath(xdgRuntimePath, repoName)
	if err != nil {
		return
	}

	hash, err := v1.NewHash(hashString)
	if err != nil {
		return
	}

	image, err = idxPath.Image(hash)
	if err != nil {
		return
	}
	return
}

func idxFromRepoName(repoName, xdgRuntimePath string) (index *v1.IndexManifest, err error) {
	idxPath, err := layoutPath(xdgRuntimePath, repoName)
	if err != nil {
		return
	}

	idx, err := idxPath.ImageIndex()
	if err != nil {
		return
	}

	index, err = idx.IndexManifest()

	return
}

func layoutPath(repoName ...string) (idxPath layout.Path, err error) {
	path := filepath.Join(repoName...)
	if _, err = os.Stat(path); err != nil {
		return
	}

	return layout.Path(path), err
}

func (i *ImageIndex) SetOS(digest name.Digest, os string) error {
	return i.Handler.SetOS(digest, os)
}

func (i *ManifestHandler) SetOS(digest name.Digest, os string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupIdxMfest := mfest.DeepCopy()
		dupIdxMfest.Subject.Platform.OS = os
		manifestBytes, err := json.Marshal(dupIdxMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(hash, false, layout.WithPlatform(
			v1.Platform{
				OS: os,
			},
		))

		i.newManifest.Set(hash, manifestBytes)
		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return err
	}

	mfest = manifest.DeepCopy()
	mfest.Subject.Platform.OS = os
	manifestBytes, err := json.Marshal(mfest)
	if err != nil {
		return err
	}

	i.instance.Replace(hash, false, layout.WithPlatform(
		v1.Platform{
			OS: os,
		},
	))

	i.newManifest.Set(hash, manifestBytes)
	return nil
}

func (i *ImageIndex) SetArchitecture(digest name.Digest, arch string) error {
	return i.Handler.SetArchitecture(digest, arch)
}

func (i *ManifestHandler) SetArchitecture(digest name.Digest, arch string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.Platform.Architecture = arch
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(hash, false, layout.WithPlatform(
			v1.Platform{
				Architecture: arch,
			},
		))
		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	mfest, err = img.Manifest()
	if err != nil {
		return err
	}

	dupMfest := mfest.DeepCopy()
	dupMfest.Subject.Platform.Architecture = arch
	manifestBytes, err := json.Marshal(dupMfest)
	if err != nil {
		return err
	}

	i.instance.Replace(hash, false, layout.WithPlatform(
		v1.Platform{
			Architecture: arch,
		},
	))
	i.newManifest.Set(hash, manifestBytes)

	return nil
}

func (i *ImageIndex) SetVariant(digest name.Digest, osVariant string) error {
	return i.Handler.SetVariant(digest, osVariant)
}

func (i *ManifestHandler) SetVariant(digest name.Digest, osVariant string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.Platform.Variant = osVariant
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			false,
			layout.WithPlatform(
				v1.Platform{
					Variant: osVariant,
				},
			),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	mfest, err = img.Manifest()
	if err != nil {
		return err
	}

	dupMfest := mfest.DeepCopy()
	dupMfest.Subject.Platform.Variant = osVariant
	manifestBytes, err := json.Marshal(dupMfest)
	if err != nil {
		return err
	}

	i.instance.Replace(
		hash,
		false,
		layout.WithPlatform(
			v1.Platform{
				Variant: osVariant,
			},
		),
	)

	i.newManifest.Set(hash, manifestBytes)

	return nil
}

func (i *ImageIndex) SetOSVersion(digest name.Digest, osVersion string) error {
	return i.Handler.SetOSVersion(digest, osVersion)
}

func (i *ManifestHandler) SetOSVersion(digest name.Digest, osVersion string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.Platform.OSVersion = osVersion
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			false,
			layout.WithPlatform(
				v1.Platform{
					OSVersion: osVersion,
				},
			),
		)
		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	mfest, err = img.Manifest()
	if err != nil {
		return err
	}

	dupMfest := mfest.DeepCopy()
	dupMfest.Subject.Platform.OSVersion = osVersion
	manifestBytes, err := json.Marshal(dupMfest)
	if err != nil {
		return err
	}

	i.instance.Replace(
		hash,
		false,
		layout.WithPlatform(
			v1.Platform{
				OSVersion: osVersion,
			},
		),
	)
	i.newManifest.Set(hash, manifestBytes)

	return nil
}

func (i *ImageIndex) SetFeatures(digest name.Digest, features []string) error {
	return i.Handler.SetFeatures(digest, features)
}

func (i *ManifestHandler) SetFeatures(digest name.Digest, features []string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.Platform.Features = features
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			false,
			layout.WithPlatform(
				v1.Platform{
					Features: features,
				},
			),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	mfest, err = img.Manifest()
	if err != nil {
		return err
	}

	dupMfest := mfest.DeepCopy()
	dupMfest.Subject.Platform.Features = features
	manifestBytes, err := json.Marshal(dupMfest)
	if err != nil {
		return err
	}

	i.instance.Replace(
		hash,
		false,
		layout.WithPlatform(
			v1.Platform{
				Features: features,
			},
		),
	)

	i.newManifest.Set(hash, manifestBytes)
	return nil
}

func (i *ImageIndex) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	return i.Handler.SetOSFeatures(digest, osFeatures)
}

func (i *ManifestHandler) SetOSFeatures(digest name.Digest, osFeatures []string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.Platform.OSFeatures = osFeatures
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			false,
			layout.WithPlatform(
				v1.Platform{
					OSFeatures: osFeatures,
				},
			),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	mfest, err = img.Manifest()
	if err != nil {
		return err
	}

	dupMfest := mfest.DeepCopy()
	dupMfest.Subject.Platform.OSFeatures = osFeatures
	manifestBytes, err := json.Marshal(dupMfest)
	if err != nil {
		return err
	}

	i.instance.Replace(
		hash,
		false,
		layout.WithPlatform(
			v1.Platform{
				OSFeatures: osFeatures,
			},
		),
	)

	i.newManifest.Set(hash, manifestBytes)
	return nil
}

func (i *ImageIndex) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	return i.Handler.SetAnnotations(digest, annotations)
}

func (i *ManifestHandler) SetAnnotations(digest name.Digest, annotations map[string]string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfestIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		dupMfestIdx := mfestIdx.DeepCopy()
		dupMfestIdx.Subject.Annotations = annotations
		dupMfestIdx.Annotations = annotations
		manifestBytes, err := json.Marshal(dupMfestIdx)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			true,
			layout.WithAnnotations(annotations),
		)
		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.Annotations = annotations
		dupMfest.Annotations = annotations
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			false,
			layout.WithAnnotations(annotations),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	idx, err := path.ImageIndex()
	if err == nil {
		if h, _ := idx.Digest(); h == hash {
			idxMfest, err := idx.IndexManifest()
			if err != nil {
				return err
			}

			dupIdxMfest := idxMfest.DeepCopy()
			dupIdxMfest.Subject.Annotations = annotations
			dupIdxMfest.Annotations = annotations
			manifestBytes, err := json.Marshal(dupIdxMfest)
			if err != nil {
				return err
			}

			i.instance.Replace(
				hash,
				false,
				layout.WithAnnotations(annotations),
			)

			i.newManifest.Set(hash, manifestBytes)

			return nil
		}

		imgImg, err := idx.ImageIndex(hash)
		if err != nil {
			return err
		}

		idxMfest, err := imgImg.IndexManifest()
		if err != nil {
			return err
		}

		dupIdxMfest := idxMfest.DeepCopy()
		dupIdxMfest.Annotations = annotations
		dupIdxMfest.Subject.Annotations = annotations
		manifestBytes, err := json.Marshal(dupIdxMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			true,
			layout.WithAnnotations(annotations),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	mfest, err = img.Manifest()
	if err != nil {
		return err
	}

	dupMfest := mfest.DeepCopy()
	dupMfest.Subject.Annotations = annotations
	dupMfest.Annotations = annotations
	manifestBytes, err := json.Marshal(dupMfest)
	if err != nil {
		return err
	}

	i.instance.Replace(
		hash,
		false,
		layout.WithAnnotations(annotations),
	)
	i.newManifest.Set(hash, manifestBytes)

	return nil
}

func (i *ImageIndex) SetURLs(digest name.Digest, urls []string) error {
	return i.Handler.SetURLs(digest, urls)
}

func (i *ManifestHandler) SetURLs(digest name.Digest, urls []string) error {
	digestStr := digest.Identifier()
	hash, err := v1.NewHash(digestStr)
	if err != nil {
		return err
	}

	mfestIdx, err := i.newManifest.IndexManifest(hash)
	if err == nil {
		dupMfestIdx := mfestIdx.DeepCopy()
		dupMfestIdx.Subject.URLs = urls
		manifestBytes, err := json.Marshal(dupMfestIdx)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			true,
			layout.WithURLs(urls),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	mfest, err := i.newManifest.Manifest(hash)
	if err == nil {
		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.URLs = urls
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			false,
			layout.WithURLs(urls),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	imgIdx, err := path.ImageIndex()
	if err == nil {
		if h, _ := imgIdx.Digest(); h == hash {
			mfest, err := imgIdx.IndexManifest()
			if err != nil {
				return err
			}

			dupMfest := mfest.DeepCopy()
			dupMfest.Subject.URLs = urls
			manifestBytes, err := json.Marshal(dupMfest)
			if err != nil {
				return err
			}

			i.instance.Replace(
				hash,
				true,
				layout.WithURLs(urls),
			)

			i.newManifest.Set(hash, manifestBytes)

			return nil
		}

		idx, err := imgIdx.ImageIndex(hash)
		if err != nil {
			return err
		}

		mfest, err := idx.IndexManifest()
		if err != nil {
			return err
		}

		dupMfest := mfest.DeepCopy()
		dupMfest.Subject.URLs = urls
		manifestBytes, err := json.Marshal(dupMfest)
		if err != nil {
			return err
		}

		i.instance.Replace(
			hash,
			true,
			layout.WithURLs(urls),
		)

		i.newManifest.Set(hash, manifestBytes)

		return nil
	}

	img, err := path.Image(hash)
	if err != nil {
		return err
	}

	mfest, err = img.Manifest()
	if err != nil {
		return err
	}

	dupMfest := mfest.DeepCopy()
	dupMfest.Subject.URLs = urls
	manifestBytes, err := json.Marshal(dupMfest)
	if err != nil {
		return err
	}

	i.instance.Replace(
		hash,
		false,
		layout.WithURLs(urls),
	)

	i.newManifest.Set(hash, manifestBytes)

	return nil
}

func (i *ImageIndex) Add(ref name.Reference, ops ...IndexAddOption) error {
	return i.Handler.Add(ref, ops...)
}

func (i *ManifestHandler) Add(ref name.Reference, ops ...IndexAddOption) error {
	var opts = IndexAddOptions{}
	for _, op := range ops {
		op(&opts)
	}

	desc, err := remote.Head(ref, remote.WithAuthFromKeychain(i.keychain))
	if err != nil {
		return err
	}

	descManifestBytes, err := json.MarshalIndent(desc, "", "   ")
	if err != nil {
		return err
	}

	switch {
	case desc.MediaType.IsImage():
		if opts.all {
			fmt.Printf("ignoring `-all`, ref: %s is Image", ref.Name())
		}

		err := i.instance.AddDescriptor(desc, opts.LayoutOptions()...)
		if err != nil {
			return err
		}

		i.newManifest.Set(desc.Digest, descManifestBytes)
	case desc.MediaType.IsIndex():
		mfestIdx, err := v1.ParseIndexManifest(bytes.NewReader(descManifestBytes))
		if err != nil {
			return err
		}

		if opts.all {
			for idx := range mfestIdx.Manifests {
				if mfestIdx.Manifests[idx].MediaType.IsImage() {
					descManifestImgBytes, err := json.MarshalIndent(mfestIdx.Manifests[idx], "", "   ")
					if err != nil {
						return err
					}

					err = i.instance.AddDescriptor(&mfestIdx.Manifests[idx], opts.LayoutOptions()...)
					if err != nil {
						return err
					}

					i.newManifest.Set(mfestIdx.Manifests[idx].Digest, descManifestImgBytes)
				}
			}

			return nil
		}

		addSingleImage := func(descManifest v1.Descriptor) error {
			descManifestImgBytes, err := json.MarshalIndent(descManifest, "", "   ")
			if err != nil {
				return err
			}

			err = i.instance.AddDescriptor(&descManifest, opts.LayoutOptions()...)
			if err != nil {
				return err
			}

			i.newManifest.Set(descManifest.Digest, descManifestImgBytes)
			return nil
		}

		if opts.os == "" || opts.arch == "" {
			for _, descManifest := range mfestIdx.Manifests {
				if (descManifest.Platform.OS == opts.os || descManifest.Platform.OS == runtime.GOOS) && (descManifest.Platform.Architecture == opts.arch || descManifest.Platform.Architecture == runtime.GOARCH) {
					return addSingleImage(descManifest)
				}

				if descManifest.Platform.OS == runtime.GOOS {
					return addSingleImage(descManifest)
				}
			}

			return fmt.Errorf("no image found in the ImageIndex with the current Platform")
		}

		var matchingDescriptor v1.Descriptor
		var bestMatchCount int

		for _, descManifest := range mfestIdx.Manifests {
			if descManifest.Platform != nil {
				continue
			}

			currentCountMatch := 0

			switch {
			case opts.os != "" && descManifest.Platform.OS == opts.os:
				currentCountMatch++
				fallthrough
			case opts.arch != "" && descManifest.Platform.Architecture == opts.arch:
				currentCountMatch++
				fallthrough
			case opts.variant != "" && descManifest.Platform.Variant == opts.variant:
				currentCountMatch++
				fallthrough
			case len(opts.features) != 0 && stringSlicesEqual(descManifest.Platform.Features, opts.features):
				currentCountMatch++
				fallthrough
			case len(opts.annotations) != 0 && reflect.DeepEqual(descManifest.Annotations, opts.annotations):
				currentCountMatch++
			}

			if currentCountMatch > bestMatchCount {
				matchingDescriptor = descManifest
				bestMatchCount = currentCountMatch
			}
		}

		if bestMatchCount == 0 {
			return fmt.Errorf("no image found with the provided options")
		}

		return addSingleImage(matchingDescriptor)
	}
	return fmt.Errorf("unexpected error occurred")
}

func (i *ImageIndex) Save() error {
	return i.Handler.Save()
}

func (i *ManifestHandler) Save() error {
	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	if m, err := path.ImageIndex(); err == nil && i.requestedMediaTypes.IndexType() == DockerTypes.IndexType() {
		idx, err := m.IndexManifest()
		if err != nil {
			return err
		}

		// Docker's ManifestList doesn't have Annotations field
		idx.Annotations = nil
		idx.Subject.Annotations = nil
	}

	for h := range *i.instance {
		for _, manifestActions := range i.instance.get(h) {
			switch manifestActions.action {
			case ADD:
				if manifestActions.descriptor.MediaType == types.DockerManifestList {
					manifestActions.descriptor.Annotations = nil
				}

				err := path.AppendDescriptor(*manifestActions.descriptor)
				if err != nil {
					return err
				}
			case REPLACE:
				err := path.RemoveDescriptors(match.Digests(manifestActions.hash))
				if err != nil {
					return err
				}

				if manifestActions.descriptor.MediaType == types.DockerManifestList {
					manifestActions.descriptor.Annotations = nil
				}

				err = path.AppendDescriptor(*manifestActions.descriptor)
				if err != nil {
					return err
				}
			case DELETE:
				err := path.RemoveDescriptors(match.Digests(manifestActions.hash))
				if err != nil {
					return err
				}
			}
		}
	}

	file, err := os.Create(filepath.Join(i.xdgRuntimePath, i.repoName, "index.map.json"))
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(i.indexMap)
}

func (i *ImageIndex) Push(ops ...IndexAddOption) error {
	return i.Handler.Push(ops...)
}

func (i *ManifestHandler) Push(ops ...IndexAddOption) error {
	var pushOpts = IndexAddOptions{}
	for _, op := range ops {
		op(&pushOpts)
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	imgIdx, err := path.ImageIndex()
	if err != nil {
		return err
	}

	mfest, err := imgIdx.IndexManifest()
	if err != nil {
		return err
	}

	if reqMediaType := pushOpts.format; mfest.MediaType != reqMediaType.IndexType() {
		i.requestedMediaTypes = reqMediaType
		if err = i.Save(); err != nil {
			return err
		}
	}

	if err = remote.WriteIndex(i.ref, imgIdx, remote.WithAuthFromKeychain(i.keychain)); err != nil {
		return err
	}

	if pushOpts.purge {
		return i.Delete()
	}

	return nil
}

func (i *ImageIndex) Inspect() error {
	return i.Handler.Inspect()
}

func (i *ManifestHandler) Inspect() error {
	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	idx, err := path.ImageIndex()
	if err != nil {
		return err
	}

	manifestBytes, err := idx.RawManifest()
	if err == nil {
		err = fmt.Errorf(string(manifestBytes))
	}
	return err
}

func (i *ImageIndex) Remove(digest name.Digest) error {
	return i.Handler.Remove(digest)
}

func (i *ManifestHandler) Remove(digest name.Digest) error {
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return err
	}

	path, err := layoutPath(i.xdgRuntimePath, i.repoName)
	if err != nil {
		return err
	}

	imgIdx, err := path.ImageIndex()
	if err != nil {
		return err
	}

	_, err = imgIdx.ImageIndex(hash)
	if err == nil {
		i.instance.Remove(hash, true)
		i.newManifest.Delete(hash)
	}

	_, err = imgIdx.Image(hash)
	if err == nil {
		i.instance.Remove(hash, false)
		i.newManifest.Delete(hash)
	}

	return err
}

func (i *ImageIndex) Delete() error {
	return i.Handler.Delete()
}

func (i *ManifestHandler) Delete() error {
	err := os.Remove(filepath.Join(i.xdgRuntimePath, i.repoName, "index.json"))
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(i.xdgRuntimePath, i.repoName, "index.map.json"))
	if err != nil {
		return err
	}

	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
