package index

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Annotate a helper struct used for keeping track of changes made to ImageIndex.
type Annotate struct {
	Instance map[v1.Hash]v1.Descriptor
}

// OS returns `OS` of an existing manipulated ImageIndex if found, else an error.
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

// SetOS sets the `OS` of an Image/ImageIndex to keep track of changes.
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

// Architecture returns `Architecture` of an existing manipulated ImageIndex if found, else an error.
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

// SetArchitecture annotates the `Architecture` of the given Image.
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

// Variant returns `Variant` of an existing manipulated ImageIndex if found, else an error.
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

// SetVariant annotates the `Variant` of the given Image.
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

// OSVersion returns `OSVersion` of an existing manipulated ImageIndex if found, else an error.
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

// SetOSVersion annotates the `OSVersion` of the given Image.
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

// Features returns `Features` of an existing manipulated ImageIndex if found, else an error.
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

// SetFeatures annotates the `Features` of the given Image.
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

// OSFeatures returns `OSFeatures` of an existing manipulated ImageIndex if found, else an error.
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

// SetOSFeatures annotates the `OSFeatures` of the given Image.
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

// Annotations returns `Annotations` of an existing manipulated ImageIndex if found, else an error.
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

// SetAnnotations annotates the `Annotations` of the given Image.
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

// URLs returns `URLs` of an existing manipulated ImageIndex if found, else an error.
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

// SetURLs annotates the `URLs` of the given Image.
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

// Format returns `types.MediaType` of an existing manipulated ImageIndex if found, else an error.
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

// SetFormat stores the `Format` of the given Image.
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
