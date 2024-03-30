package imgutil_test

import (
	"encoding/json"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/fakes"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestUtils(t *testing.T) {
	spec.Run(t, "Utils", testUtils, spec.Sequential(), spec.Report(report.Terminal{}))
}

type FakeIndentifier struct {
	hash string
}

func NewFakeIdentifier(hash string) FakeIndentifier {
	return FakeIndentifier{
		hash: hash,
	}
}

func (f FakeIndentifier) String() string {
	return f.hash
}

func testUtils(t *testing.T, when spec.G, it spec.S) {
	const fakeHash = "sha256:13553267bf712ee37527bdbbde41115b287062b72e2d54c573edf68d88e3cb4f"
	when("#MutateManifest", func() {
		var (
			img *fakes.Image
		)
		it.Before(func() {
			img = fakes.NewImage("some-name", fakeHash, NewFakeIdentifier(fakeHash))
		})
		it("should muatet Image", func() {
			var (
				annotations = map[string]string{"some-key": "some-value"}
				urls        = []string{"some-url1", "some-url2"}
				os          = "some-os"
				arch        = "some-arch"
				variant     = "some-variant"
				osVersion   = "some-os-version"
				features    = []string{"some-feat1", "some-feat2"}
				osFeatures  = []string{"some-os-feat1", "some-os-feat2"}
			)

			exptConfig, err := img.ConfigFile()
			h.AssertNil(t, err)
			h.AssertNotNil(t, exptConfig)

			img, err := imgutil.MutateManifest(img, func(c *v1.Manifest) {
				c.Annotations = annotations
				c.Config.URLs = urls
				c.Config.Platform.OS = os
				c.Config.Platform.Architecture = arch
				c.Config.Platform.Variant = variant
				c.Config.Platform.OSVersion = osVersion
				c.Config.Platform.Features = features
				c.Config.Platform.OSFeatures = osFeatures
			})

			h.AssertNil(t, err)
			mfest, err := img.Manifest()
			h.AssertNil(t, err)
			h.AssertNotNil(t, mfest)

			h.AssertEq(t, mfest.Annotations, annotations)
			h.AssertEq(t, mfest.Subject.URLs, urls)
			h.AssertEq(t, mfest.Subject.Platform.OS, os)
			h.AssertEq(t, mfest.Subject.Platform.Architecture, arch)
			h.AssertEq(t, mfest.Subject.Platform.Variant, variant)
			h.AssertEq(t, mfest.Subject.Platform.OSVersion, osVersion)
			h.AssertEq(t, mfest.Subject.Platform.Features, features)
			h.AssertEq(t, mfest.Subject.Platform.OSFeatures, osFeatures)

			orgConfig, err := img.ConfigFile()
			h.AssertNil(t, err)
			h.AssertNotNil(t, orgConfig)

			h.AssertEq(t, orgConfig, exptConfig)
		})
	})
	when("#TaggableIndex", func() {
		var (
			taggableIndex *imgutil.TaggableIndex
			amd64Hash, _  = v1.NewHash("sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34")
			armv6Hash, _  = v1.NewHash("sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a")
			indexManifest = v1.IndexManifest{
				SchemaVersion: 2,
				MediaType:     types.OCIImageIndex,
				Annotations: map[string]string{
					"test-key": "test-value",
				},
				Manifests: []v1.Descriptor{
					{
						MediaType: types.OCIManifestSchema1,
						Size:      832,
						Digest:    amd64Hash,
						Platform: &v1.Platform{
							OS:           "linux",
							Architecture: "amd64",
						},
					},
					{
						MediaType: types.OCIManifestSchema1,
						Size:      926,
						Digest:    armv6Hash,
						Platform: &v1.Platform{
							OS:           "linux",
							Architecture: "arm",
							OSVersion:    "v6",
						},
					},
				},
			}
		)
		it.Before(func() {
			taggableIndex = imgutil.NewTaggableIndex(indexManifest)
		})
		it("should return RawManifest in expected format", func() {
			mfestBytes, err := taggableIndex.RawManifest()
			h.AssertNil(t, err)

			expectedMfestBytes, err := json.Marshal(indexManifest)
			h.AssertNil(t, err)

			h.AssertEq(t, mfestBytes, expectedMfestBytes)
		})
		it("should return expected digest", func() {
			digest, err := taggableIndex.Digest()
			h.AssertNil(t, err)
			h.AssertEq(t, digest.String(), "sha256:2375c0dfd06dd51b313fd97df5ecf3b175380e895287dd9eb2240b13eb0b5703")
		})
		it("should return expected size", func() {
			size, err := taggableIndex.Size()
			h.AssertNil(t, err)
			h.AssertEq(t, size, int64(547))
		})
		it("should return expected media type", func() {
			format, err := taggableIndex.MediaType()
			h.AssertNil(t, err)
			h.AssertEq(t, format, indexManifest.MediaType)
		})
	})
	when("#StringSet", func() {
		var (
			stringSet *imgutil.StringSet
		)
		it.Before(func() {
			stringSet = imgutil.NewStringSet()
		})
		it("should add items", func() {
			item := "item1"
			stringSet.Add(item)

			h.AssertEq(t, stringSet.StringSlice(), []string{item})
		})
		it("should remove item", func() {
			item := "item1"
			stringSet.Add(item)

			h.AssertEq(t, stringSet.StringSlice(), []string{item})

			stringSet.Remove(item)
			h.AssertEq(t, stringSet.StringSlice(), []string(nil))
		})
		it("should return added items", func() {
			items := []string{"item1", "item2", "item3"}
			for _, item := range items {
				stringSet.Add(item)
			}

			h.AssertEq(t, stringSet.StringSlice(), items)
		})
		it("should not support duplicates", func() {
			item1 := "item1"
			item2 := "item2"
			items := []string{item1, item2, item1}
			for _, item := range items {
				stringSet.Add(item)
			}

			h.AssertEq(t, stringSet.StringSlice(), []string{item1, item2})
		})
	})
	when("Annotate", func() {
		annotate := imgutil.Annotate{
			Instance: map[v1.Hash]v1.Descriptor{},
		}
		it.Before(func() {
			annotate = imgutil.Annotate{
				Instance: map[v1.Hash]v1.Descriptor{},
			}
		})
		when("#OS", func() {
			it.Before(func() {
				annotate.SetOS(v1.Hash{}, "some-os")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetOS(v1.Hash{}, "")
				os, err := annotate.OS(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, os, "")
			})
			it("should return expected os", func() {
				os, err := annotate.OS(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
		})
		when("#Architecture", func() {
			it.Before(func() {
				annotate.SetArchitecture(v1.Hash{}, "some-arch")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetArchitecture(v1.Hash{}, "")
				arch, err := annotate.Architecture(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, arch, "")
			})
			it("should return expected os", func() {
				arch, err := annotate.Architecture(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "some-arch")
			})
		})
		when("#Variant", func() {
			it.Before(func() {
				annotate.SetVariant(v1.Hash{}, "some-variant")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetVariant(v1.Hash{}, "")
				variant, err := annotate.Variant(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, variant, "")
			})
			it("should return expected os", func() {
				variant, err := annotate.Variant(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "some-variant")
			})
		})
		when("#OSVersion", func() {
			it.Before(func() {
				annotate.SetOSVersion(v1.Hash{}, "some-osVersion")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetOSVersion(v1.Hash{}, "")
				osVersion, err := annotate.OSVersion(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, osVersion, "")
			})
			it("should return expected os", func() {
				osVersion, err := annotate.OSVersion(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, osVersion, "some-osVersion")
			})
		})
		when("#Features", func() {
			it.Before(func() {
				annotate.SetFeatures(v1.Hash{}, []string{"some-features"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetFeatures(v1.Hash{}, []string(nil))
				features, err := annotate.Features(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, features, []string(nil))
			})
			it("should return expected features", func() {
				os, err := annotate.Features(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, os, []string{"some-features"})
			})
		})
		when("#OSFeatures", func() {
			it.Before(func() {
				annotate.SetOSFeatures(v1.Hash{}, []string{"some-osFeatures"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetOSFeatures(v1.Hash{}, []string(nil))
				osFeatures, err := annotate.OSFeatures(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, osFeatures, []string(nil))
			})
			it("should return expected os", func() {
				osFeatures, err := annotate.OSFeatures(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{"some-osFeatures"})
			})
		})
		when("#Annotations", func() {
			it.Before(func() {
				annotate.SetAnnotations(v1.Hash{}, map[string]string{"some-key": "some-value"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetAnnotations(v1.Hash{}, map[string]string(nil))
				annotations, err := annotate.Annotations(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, annotations, map[string]string(nil))
			})
			it("should return expected os", func() {
				annotations, err := annotate.Annotations(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, annotations, map[string]string{"some-key": "some-value"})
			})
		})
		when("#URLs", func() {
			it.Before(func() {
				annotate.SetURLs(v1.Hash{}, []string{"some-urls"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetURLs(v1.Hash{}, []string(nil))
				urls, err := annotate.URLs(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, urls, []string(nil))
			})
			it("should return expected os", func() {
				os, err := annotate.URLs(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, os, []string{"some-urls"})
			})
		})
		when("#Format", func() {
			it.Before(func() {
				annotate.SetFormat(v1.Hash{}, types.OCIImageIndex)
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
				h.AssertEq(t, desc.MediaType, types.OCIImageIndex)
			})
			it("should return an error", func() {
				annotate.SetFormat(v1.Hash{}, types.MediaType(""))
				format, err := annotate.Format(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, format, types.MediaType(""))
			})
			it("should return expected os", func() {
				format, err := annotate.Format(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, format, types.OCIImageIndex)
			})
		})
	})
}
