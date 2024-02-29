package fakes_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/fakes"

	h "github.com/buildpacks/imgutil/testhelpers"
)

const digestDelim = "@"

func TestFakeIndex(t *testing.T) {
	spec.Run(t, "IndexTest", fakeIndex, spec.Parallel(), spec.Report(report.Terminal{}))
}

func fakeIndex(t *testing.T, when spec.G, it spec.S) {
	var (
		fakeDigest name.Digest
		err        error
	)
	it.Before(func() {
		fakeDigest, err = name.NewDigest("busybox@sha256:d4707523ce6e12afdbe9a3be5ad69027150a834870ca0933baf7516dd1fe0f56", name.Insecure, name.WeakValidation)
		h.AssertNil(t, err)
	})
	when("#NewIndex", func() {
		it("implements imgutil.ImageIndex", func() {
			idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
			h.AssertNil(t, err)

			var _ imgutil.ImageIndex = idx
		})
		when("#NewIndex options", func() {
			when("#OS", func() {
				it("should return expected os", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						os, err := idx.OS(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						config, err := img.ConfigFile()
						h.AssertNil(t, err)
						h.AssertNotEq(t, config, nil)

						h.AssertEq(t, os, config.OS)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					os, err := idx.OS(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, os, "")
				})
			})
			when("#Architecture", func() {
				it("should return expected architecture", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample-image" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						arch, err := idx.Architecture(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						config, err := img.ConfigFile()
						h.AssertNil(t, err)
						h.AssertNotEq(t, config, nil)

						h.AssertEq(t, arch, config.Architecture)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					arch, err := idx.Architecture(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, arch, "")
				})
			})
			when("#Variant", func() {
				it("should return expected variant", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						variant, err := idx.Variant(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						config, err := img.ConfigFile()
						h.AssertNil(t, err)
						h.AssertNotEq(t, config, nil)

						h.AssertEq(t, variant, config.Variant)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					variant, err := idx.Variant(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, variant, "")
				})
			})
			when("#OSVersion", func() {
				it("should return expected os version", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						osVersion, err := idx.OSVersion(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						config, err := img.ConfigFile()
						h.AssertNil(t, err)
						h.AssertNotEq(t, config, nil)

						h.AssertEq(t, osVersion, config.OSVersion)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					osVersion, err := idx.OSVersion(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, osVersion, "")
				})
			})
			when("#Features", func() {
				it("should return expected features", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						features, err := idx.Features(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						config, err := img.ConfigFile()
						h.AssertNil(t, err)
						h.AssertNotEq(t, config, nil)

						platform := config.Platform()
						if platform == nil {
							platform = &v1.Platform{}
						}

						h.AssertEq(t, features, platform.Features)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					features, err := idx.Features(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, features, []string(nil))
				})
			})
			when("#OSFeatures", func() {
				it("should return expected os features", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						osFeatures, err := idx.OSFeatures(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						config, err := img.ConfigFile()
						h.AssertNil(t, err)
						h.AssertNotEq(t, config, nil)

						h.AssertEq(t, osFeatures, config.OSFeatures)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					osFeatures, err := idx.OSFeatures(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, osFeatures, []string(nil))
				})
			})
			when("#Annotations", func() {
				it("should return expected annotations for oci", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						annotations, err := idx.Annotations(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						mfest, err := img.Manifest()
						h.AssertNil(t, err)
						if mfest == nil {
							mfest = &v1.Manifest{}
						}

						h.AssertEq(t, annotations, mfest.Annotations)
					}
				})
				it("should not return annotations for docker", func() {
					idx, err := fakes.NewIndex(types.DockerManifestList, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						annotations, err := idx.Annotations(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, annotations, map[string]string(nil))
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					annos, err := idx.Annotations(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, annos, map[string]string(nil))
				})
			})
			when("#URLs", func() {
				it("should return expected urls", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						urls, err := idx.URLs(digest)
						h.AssertNil(t, err)

						img, err := idx.Image(mfest.Digest)
						h.AssertNil(t, err)

						mfest, err := img.Manifest()
						h.AssertNil(t, err)

						if mfest == nil {
							mfest = &v1.Manifest{}
						}

						h.AssertEq(t, urls, mfest.Config.URLs)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					urls, err := idx.URLs(fakeDigest)
					h.AssertNotEq(t, err, nil)
					h.AssertEq(t, urls, []string(nil))
				})
			})
			when("#SetOS", func() {
				it("should annotate the image os", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := "some-os"
					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						err = idx.SetOS(digest, annotated)
						h.AssertNil(t, err)

						os, err := idx.OS(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, os, annotated)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetOS(fakeDigest, "")
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#SetArchitecture", func() {
				it("should annotate the image architecture", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := "some-arch"
					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						err = idx.SetArchitecture(digest, annotated)
						h.AssertNil(t, err)

						arch, err := idx.Architecture(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, arch, annotated)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetArchitecture(fakeDigest, "")
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#SetVariant", func() {
				it("should annotate the image variant", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := "some-variant"
					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						err = idx.SetVariant(digest, annotated)
						h.AssertNil(t, err)

						variant, err := idx.Variant(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, variant, annotated)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetVariant(fakeDigest, "")
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#SetOSVersion", func() {
				it("should annotate the image os version", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := "some-os-version"
					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						err = idx.SetOSVersion(digest, annotated)
						h.AssertNil(t, err)

						osVersion, err := idx.OSVersion(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, osVersion, annotated)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetOSVersion(fakeDigest, "")
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#SetFeatures", func() {
				it("should annotate the features", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := []string{"some-feature"}
					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						err = idx.SetFeatures(digest, annotated)
						h.AssertNil(t, err)

						features, err := idx.Features(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, features, annotated)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetFeatures(fakeDigest, []string{""})
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#SetOSFeatures", func() {
				it("should annotate the os features", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := []string{"some-os-feature"}
					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						err = idx.SetOSFeatures(digest, annotated)
						h.AssertNil(t, err)

						osFeatures, err := idx.OSFeatures(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, osFeatures, annotated)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetOSFeatures(fakeDigest, []string{""})
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#SetAnnotations", func() {
				it("should annotate the annotations", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := map[string]string{"some-key": "some-value"}
					digest, err := name.NewDigest("cnbs/sample" + digestDelim + idxMfest.Manifests[0].Digest.String())
					h.AssertNil(t, err)

					err = idx.SetAnnotations(digest, annotated)
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, annotations, annotated)
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetAnnotations(fakeDigest, map[string]string{"some-key": "some-value"})
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#SetURLs", func() {
				it("should annotate the urls", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					idxMfest, err := idx.IndexManifest()
					h.AssertNil(t, err)

					annotated := []string{"some-urls"}
					for _, mfest := range idxMfest.Manifests {
						digest, err := name.NewDigest("cnbs/sample" + digestDelim + mfest.Digest.String())
						h.AssertNil(t, err)

						err = idx.SetURLs(digest, annotated)
						h.AssertNil(t, err)

						urls, err := idx.URLs(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, urls, annotated)
					}
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.SetURLs(fakeDigest, []string{""})
					h.AssertNotEq(t, err, nil)
				})
			})
			when("#Add", func() {
				it("should add an image", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					digest, err := name.NewDigest("cnbs/sample-image" + digestDelim + "sha256:6d5a11994be8ca5e4cfaf4d370219f6eb6ef8fb41d57f9ed1568a93ffd5471ef")
					h.AssertNil(t, err)
					err = idx.Add(digest)
					h.AssertNil(t, err)

					_, err = idx.OS(digest)
					h.AssertNil(t, err)
				})
				it("should add from Index", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					digest, err := name.NewDigest("cnbs/sample-image" + digestDelim + "sha256:6d5a11994be8ca5e4cfaf4d370219f6eb6ef8fb41d57f9ed1568a93ffd5471ef")
					h.AssertNil(t, err)
					err = idx.Add(digest, imgutil.WithOS("some-os"), imgutil.WithArchitecture("some-arch"))
					h.AssertNil(t, err)

					os, err := idx.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "some-os")

					arch, err := idx.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "some-arch")
				})
			})
			when("#Save", func() {
				it("should save image", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)
				})
				it("should return an error", func() {})
			})
			when("#Push", func() {
				it("should push index to registry", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.Push()
					h.AssertNil(t, err)
				})
				it("should return an error", func() {})
			})
			when("#Inspect", func() {
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					mfest, err := idx.Inspect()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, "")
				})
			})
			when("#Delete", func() {
				it("should delete index from local storage", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.Delete()
					h.AssertNil(t, err)
				})
				it("should return an error", func() {
					idx, err := fakes.NewIndex(types.OCIImageIndex, 1024, 1, 1, v1.Descriptor{})
					h.AssertNil(t, err)

					err = idx.Delete()
					h.AssertNil(t, err)

					err = idx.Delete()
					h.AssertNotEq(t, err, nil)
				})
			})
		})
	})
}
