package imgutil

import (
	"encoding/json"
	"slices"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
)

func GetConfigFile(image v1.Image) (*v1.ConfigFile, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}
	if configFile == nil {
		return nil, errors.New("missing config file")
	}
	return configFile, nil
}

func GetManifest(image v1.Image) (*v1.Manifest, error) {
	manifest, err := image.Manifest()
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		return nil, errors.New("missing manifest")
	}
	return manifest, nil
}

func MutateManifest(i v1.Image, withFunc func(c *v1.Manifest) (mutateSubject, mutateAnnoations bool)) (v1.Image, error) {
	// FIXME: put MutateManifest on the interface when `remote` and `layout` packages also support it.
	digest, err := i.Digest()
	if err != nil {
		return nil, err
	}

	mfest, err := GetManifest(i)
	if err != nil {
		return nil, err
	}

	mfest = mfest.DeepCopy()
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
	if len(mfest.Annotations) == 0 {
		mfest.Annotations = make(map[string]string)
	}

	if len(mfest.Config.Annotations) == 0 {
		mfest.Config.Annotations = make(map[string]string)
	}

	mutateSub, mutateAnnos := withFunc(mfest)
	if mutateAnnos {
		i = mutate.Annotations(i, mfest.Annotations).(v1.Image)
	}

	if mutateSub {
		i = mutate.Subject(i, mfest.Config).(v1.Image)
	}

	return i, err
}

func MutateManifestFn(mfest *v1.Manifest, os, arch, variant, osVersion string, features, osFeatures, urls []string, annotations map[string]string) (mutateSubject, mutateAnnotations bool) {
	config := mfest.Config
	if len(annotations) != 0 && !(MapContains(mfest.Annotations, annotations) || MapContains(config.Annotations, annotations)) {
		mutateAnnotations = true
		for k, v := range annotations {
			mfest.Annotations[k] = v
			config.Annotations[k] = v
		}
	}

	if len(urls) != 0 && !SliceContains(config.URLs, urls) {
		mutateSubject = true
		stringSet := NewStringSet()
		for _, value := range config.URLs {
			stringSet.Add(value)
		}
		for _, value := range urls {
			stringSet.Add(value)
		}

		config.URLs = stringSet.StringSlice()
	}

	if config.Platform == nil {
		config.Platform = &v1.Platform{}
	}

	if len(features) != 0 && !SliceContains(config.Platform.Features, features) {
		mutateSubject = true
		stringSet := NewStringSet()
		for _, value := range config.Platform.Features {
			stringSet.Add(value)
		}
		for _, value := range features {
			stringSet.Add(value)
		}

		config.Platform.Features = stringSet.StringSlice()
	}

	if len(osFeatures) != 0 && !SliceContains(config.Platform.OSFeatures, osFeatures) {
		mutateSubject = true
		stringSet := NewStringSet()
		for _, value := range config.Platform.OSFeatures {
			stringSet.Add(value)
		}
		for _, value := range osFeatures {
			stringSet.Add(value)
		}

		config.Platform.OSFeatures = stringSet.StringSlice()
	}

	if os != "" && config.Platform.OS != os {
		mutateSubject = true
		config.Platform.OS = os
	}

	if arch != "" && config.Platform.Architecture != arch {
		mutateSubject = true
		config.Platform.Architecture = arch
	}

	if variant != "" && config.Platform.Variant != variant {
		mutateSubject = true
		config.Platform.Variant = variant
	}

	if osVersion != "" && config.Platform.OSVersion != osVersion {
		mutateSubject = true
		config.Platform.OSVersion = osVersion
	}

	mfest.Config = config
	return mutateSubject, mutateAnnotations
}

// TaggableIndex any ImageIndex with RawManifest method.
type TaggableIndex struct {
	*v1.IndexManifest
}

// RawManifest returns the bytes of IndexManifest.
func (t *TaggableIndex) RawManifest() ([]byte, error) {
	return json.Marshal(t.IndexManifest)
}

// Digest returns the Digest of the IndexManifest if present.
// Else generate a new Digest.
func (t *TaggableIndex) Digest() (v1.Hash, error) {
	if t.IndexManifest.Subject != nil && t.IndexManifest.Subject.Digest != (v1.Hash{}) {
		return t.IndexManifest.Subject.Digest, nil
	}

	return partial.Digest(t)
}

// MediaType returns the MediaType of the IndexManifest.
func (t *TaggableIndex) MediaType() (types.MediaType, error) {
	return t.IndexManifest.MediaType, nil
}

// Size returns the Size of IndexManifest if present.
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

func NewStringSet() *StringSet {
	return &StringSet{items: make(map[string]bool)}
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

func MapContains(src, target map[string]string) bool {
	for targetKey, targetValue := range target {
		if value := src[targetKey]; targetValue == value {
			continue
		}
		return false
	}
	return true
}

func SliceContains(src, target []string) bool {
	for _, value := range target {
		if ok := slices.Contains[[]string, string](src, value); !ok {
			return false
		}
	}
	return true
}

// MakeFileSafeName Change a reference name string into a valid file name
// Ex: cnbs/sample-package:hello-multiarch-universe
// to cnbs_sample-package-hello-multiarch-universe
func MakeFileSafeName(ref string) string {
	fileName := strings.ReplaceAll(ref, ":", "-")
	return strings.ReplaceAll(fileName, "/", "_")
}

func NewEmptyDockerIndex() v1.ImageIndex {
	idx := empty.Index
	return mutate.IndexMediaType(idx, types.DockerManifestList)
}
