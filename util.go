package imgutil

import (
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func MutateManifest(i v1.Image, withFunc func(c *v1.Manifest)) (v1.Image, error) {
	// FIXME: put MutateManifest on the interface when `remote` and `layout` packages also support it.
	digest, err := i.Digest()
	if err != nil {
		return nil, err
	}

	mfest, err := getManifest(i)
	if err != nil {
		return nil, err
	}

	config := mfest.Config
	config.Digest = digest
	config.MediaType = mfest.MediaType
	if config.Size, err = partial.Size(i); err != nil {
		return nil, err
	}
	config.Annotations = mfest.Annotations

	p := config.Platform
	if p == nil {
		p = &v1.Platform{}
	}

	config.Platform = p
	mfest.Config = config

	withFunc(mfest)
	if len(mfest.Annotations) != 0 {
		i = mutate.Annotations(i, mfest.Annotations).(v1.Image)
	}

	return mutate.Subject(i, mfest.Config).(v1.Image), err
}

// Any ImageIndex with RawManifest method.
type TaggableIndex struct {
	*v1.IndexManifest
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

type StringSet struct {
	items map[string]bool
}

func (s *StringSet) Add(str string) {
	if s == nil {
		s = &StringSet{items: make(map[string]bool)}
	}

	s.items[str] = true
}

func (s *StringSet) Remove(str string) {
	if s == nil {
		s = &StringSet{items: make(map[string]bool)}
	}

	s.items[str] = false
}

func (s *StringSet) StringSlice() (slice []string) {
	if s == nil {
		s = &StringSet{items: make(map[string]bool)}
	}

	for i, ok := range s.items {
		if ok {
			slice = append(slice, i)
		}
	}

	return slice
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
		return os, ErrOSUndefined(types.DockerConfigJSON, hash.String())
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
		return arch, ErrArchUndefined(types.DockerConfigJSON, hash.String())
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
		return variant, ErrVariantUndefined(types.DockerConfigJSON, hash.String())
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
		return osVersion, ErrOSVersionUndefined(types.DockerConfigJSON, hash.String())
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
		return features, ErrFeaturesUndefined(types.DockerConfigJSON, hash.String())
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
		return osFeatures, ErrOSFeaturesUndefined(types.DockerConfigJSON, hash.String())
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
		return annotations, ErrAnnotationsUndefined(types.DockerConfigJSON, hash.String())
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
		return urls, ErrURLsUndefined(types.DockerConfigJSON, hash.String())
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
		return format, ErrUnknownMediaType(desc.MediaType)
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

// Annotate and Append Manifests to ImageIndex.
func appendAnnotatedManifests(desc v1.Descriptor, imgDesc v1.Descriptor, path layout.Path, errs *SaveError) {
	if len(desc.Annotations) != 0 && (imgDesc.MediaType == types.OCIImageIndex || imgDesc.MediaType == types.OCIManifestSchema1) {
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
}

func parseReferenceToHash(ref name.Reference, options IndexOptions) (hash v1.Hash, err error) {
	switch v := ref.(type) {
	case name.Tag:
		desc, err := remote.Head(
			v,
			remote.WithAuthFromKeychain(options.KeyChain),
			remote.WithTransport(
				GetTransport(options.InsecureRegistry),
			),
		)
		if err != nil {
			return hash, err
		}

		if desc == nil {
			return hash, ErrManifestUndefined
		}

		hash = desc.Digest
	default:
		hash, err = v1.NewHash(v.Identifier())
		if err != nil {
			return hash, err
		}
	}

	return hash, nil
}

func getIndexManifest(ii v1.ImageIndex) (mfest *v1.IndexManifest, err error) {
	mfest, err = ii.IndexManifest()
	if mfest == nil {
		return mfest, ErrManifestUndefined
	}

	return mfest, err
}

func indexMediaType(format types.MediaType) string {
	switch format {
	case types.DockerManifestList, types.DockerManifestSchema2:
		return "Docker"
	case types.OCIImageIndex, types.OCIManifestSchema1:
		return "OCI"
	default:
		return "UNKNOWN"
	}
}
