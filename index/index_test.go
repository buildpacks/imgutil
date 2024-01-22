package index_test

import (
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/index"
	"github.com/buildpacks/imgutil/layout"
	"github.com/buildpacks/imgutil/local"
	"github.com/buildpacks/imgutil/remote"

	h "github.com/buildpacks/imgutil/testhelpers"
)

func TextIndex(t *testing.T) {
	spec.Run(t, "IndexTest", testIndex, spec.Sequential(), spec.Report(report.Terminal{}))
}

const (
	indexName = "alpine:3.19.0"
	xdgPath   = "xdgPath"
)

type PlatformSpecificImage struct {
	OS, Arch, Variant, OSVersion, Hash string
	Features, OSFeatures, URLs         []string
	Annotations                        map[string]string
	Found                              bool
}

func testIndex(t *testing.T, when spec.G, it spec.S) {
	when("#NewIndex", func() {
		it.Before(func() {
			idx, err := remote.NewIndex(indexName, index.WithKeychain(authn.DefaultKeychain), index.WithXDGRuntimePath(xdgPath))
			h.AssertNil(t, err)

			err = idx.Save()
			h.AssertNil(t, err)
		})
		it.After(func() {
			err := os.RemoveAll(xdgPath)
			h.AssertNil(t, err)
		})
		it("should create new Index", func() {
			idx, err := index.NewIndex(indexName, index.WithKeychain(authn.DefaultKeychain), index.WithXDGRuntimePath(xdgPath))
			h.AssertNil(t, err)
			h.AssertNotEq(t, idx, imgutil.Index{})
		})
		it("should return an error", func() {
			_, err := index.NewIndex(indexName+"$invalid", index.WithKeychain(authn.DefaultKeychain), index.WithXDGRuntimePath(xdgPath))
			h.AssertNotEq(t, err, nil)
		})
		when("#NewIndex options", func() {
			var (
				idx                  imgutil.ImageIndex
				err                  error
				alpineImageDigest    name.Digest
				alpineImageDigestStr = "sha256:a70bcfbd89c9620d4085f6bc2a3e2eef32e8f3cdf5a90e35a1f95dcbd7f71548"
				aplineImageOS        = "linux"
				alpineImageArch      = "arm64"
				alpineImageVariant   = "v8"
				digestDelim          = "@"
			)
			it.Before(func() {
				idx, err = remote.NewIndex(indexName, index.WithKeychain(authn.DefaultKeychain), index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, imgutil.Index{})

				err = idx.Save()
				h.AssertNil(t, err)

				alpineImageDigest, err = name.NewDigest("alpine"+digestDelim+alpineImageDigestStr, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				idx, err = index.NewIndex(indexName, index.WithKeychain(authn.DefaultKeychain), index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, imgutil.Index{})
			})
			it.After(func() {
				err = os.RemoveAll(xdgPath)
				h.AssertNil(t, err)
			})
			when("#OS", func() {
				it("should return expected os", func() {
					os, err := idx.OS(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertNil(t, err)
					h.AssertEq(t, os, aplineImageOS)
				})
				it("should return an error", func() {})
			})
			when("#Architecture", func() {
				it("should return expected architecture", func() {
					arch, err := idx.Architecture(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertNil(t, err)
					h.AssertEq(t, arch, alpineImageArch)
				})
				it("should return an error", func() {})
			})
			when("#Variant", func() {
				it("should return expected variant", func() {
					variant, err := idx.Variant(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertNil(t, err)
					h.AssertEq(t, variant, alpineImageVariant)
				})
				it("should return an error", func() {})
			})
			when("#OSVersion", func() {
				it("should return expected os version", func() {
					osVersion, err := idx.OSVersion(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")
				})
				it("should return an error", func() {})
			})
			when("#Features", func() {
				it("should return expected features", func() {
					features, err := idx.Features(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))
				})
				it("should return an error", func() {})
			})
			when("#OSFeatures", func() {
				it("should return expected os features for image", func() {
					osFeatures, err := idx.OSFeatures(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))
				})
				it("should return an error", func() {})
			})
			when("#Annotations", func() {
				it("should return expected annotations for oci", func() {})
				it("should not return annotations for docker image", func() {
					annotations, err := idx.Annotations(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return an error", func() {})
			})
			when("#URLs", func() {
				it("should return expected urls for index", func() {
					urls, err := idx.URLs(alpineImageDigest.Context().Digest(alpineImageDigestStr))
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))
				})
				it("should return expected urls for image", func() {})
				it("should return an error", func() {})
			})
			when("#SetOS", func() {
				it("should annotate the image os", func() {
					var (
						digest     = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedOS = "some-os"
					)
					err = idx.SetOS(digest, modifiedOS)
					h.AssertNil(t, err)

					os, err := idx.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, modifiedOS)
				})
				it("should return an error", func() {})
			})
			when("#SetArchitecture", func() {
				it("should annotate the image architecture", func() {
					var (
						digest       = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedArch = "some-arch"
					)
					err = idx.SetArchitecture(digest, modifiedArch)
					h.AssertNil(t, err)

					arch, err := idx.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, modifiedArch)
				})
				it("should return an error", func() {})
			})
			when("#SetVariant", func() {
				it("should annotate the image variant", func() {
					var (
						digest          = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedVariant = "some-variant"
					)
					err = idx.SetVariant(digest, modifiedVariant)
					h.AssertNil(t, err)

					variant, err := idx.Variant(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, variant, modifiedVariant)
				})
				it("should return an error", func() {})
			})
			when("#SetOSVersion", func() {
				it("should annotate the image os version", func() {
					var (
						digest            = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedOSVersion = "some-osVersion"
					)
					err = idx.SetOSVersion(digest, modifiedOSVersion)
					h.AssertNil(t, err)

					osVersion, err := idx.OSVersion(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, osVersion, modifiedOSVersion)
				})
				it("should return an error", func() {})
			})
			when("#SetFeatures", func() {
				it("should annotate the image features", func() {
					var (
						digest           = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedFeatures = []string{"some-feature"}
					)
					err = idx.SetFeatures(digest, modifiedFeatures)
					h.AssertNil(t, err)

					features, err := idx.Features(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, features, modifiedFeatures)
				})
				it("should annotate the index features", func() {})
				it("should return an error", func() {})
			})
			when("#SetOSFeatures", func() {
				it("should annotate the image os features", func() {
					var (
						digest             = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedOSFeatures = []string{"some-osFeatures"}
					)
					err = idx.SetOSFeatures(digest, modifiedOSFeatures)
					h.AssertNil(t, err)

					osFeatures, err := idx.OSFeatures(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, osFeatures, modifiedOSFeatures)
				})
				it("should annotate the index os features", func() {})
				it("should return an error", func() {})
			})
			when("#SetAnnotations", func() {
				it("should annotate the image annotations", func() {
					var (
						digest              = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedAnnotations = map[string]string{"some-key": "some-value"}
					)
					err = idx.SetAnnotations(digest, modifiedAnnotations)
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, annotations, modifiedAnnotations)
				})
				it("should annotate the index annotations", func() {})
				it("should return an error", func() {})
			})
			when("#SetURLs", func() {
				it("should annotate the image urls", func() {
					var (
						digest       = alpineImageDigest.Context().Digest(alpineImageDigestStr)
						modifiedURLs = []string{"some-urls"}
					)
					err = idx.SetURLs(digest, modifiedURLs)
					h.AssertNil(t, err)

					urls, err := idx.URLs(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, urls, modifiedURLs)
				})
				it("should annotate the index urls", func() {})
				it("should return an error", func() {})
			})
			when("#Add", func() {
				it("should add an image", func() {
					var (
						digestStr   = "sha256:b31dd6ba7d28a1559be39a88c292a1a8948491b118dafd3e8139065afe55690a"
						digest      = alpineImageDigest.Context().Digest(digestStr)
						digestStrOS = "linux"
					)
					err = idx.Add(digest)
					h.AssertNil(t, err)

					os, err := idx.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, digestStrOS)
				})
				it("should add all images in index", func() {
					var (
						refStr = "alpine:3.18.5"
					)
					ref, err := name.ParseReference(refStr, name.Insecure, name.WeakValidation)
					h.AssertNil(t, err)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 7)

					err = idx.Add(ref, imgutil.WithAll(true))
					h.AssertNil(t, err)

					mfest, err = idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 14)
				})
				it("should add platform specific image", func() {
					var (
						// digestStr        = "sha256:1832ef473ede9a923cc6affdf13b54a1be6561ad2ce3c3684910260a7582d36b"
						refStr           = "alpine:3.18.5"
						digestStrOS      = "linux"
						digestStrArch    = "arm"
						digestStrVariant = "v6"
					)
					ref, err := name.ParseReference(refStr, name.Insecure, name.WeakValidation)
					h.AssertNil(t, err)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 7)

					err = idx.Add(
						ref,
						imgutil.WithOS(digestStrOS),
						imgutil.WithArchitecture(digestStrArch),
						imgutil.WithVariant(digestStrVariant),
					)
					h.AssertNil(t, err)

					mfest, err = idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 8)
				})
				it("should add target specific image", func() {
					var (
						refStr = "alpine:3.18.5"
					)
					ref, err := name.ParseReference(refStr, name.Insecure, name.WeakValidation)
					h.AssertNil(t, err)

					imgIdx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := imgIdx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						err = idx.Remove(ref.Context().Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = imgIdx.Add(ref)
					h.AssertNil(t, err)

					err = imgIdx.Save()
					h.AssertNil(t, err)

					format, err := imgIdx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err = local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)
					} else {
						idx, err = layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)
					}

					imgIdx, ok = idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err = imgIdx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						descDigest := ref.Context().Digest(m.Digest.String())
						os, err := idx.OS(descDigest)
						h.AssertNil(t, err)
						h.AssertEq(t, os, runtime.GOOS)

						arch, err := idx.Architecture(descDigest)
						h.AssertNil(t, err)
						h.AssertEq(t, arch, runtime.GOARCH)
					}
				})
				it("should return an error", func() {})
			})
			when("#Save", func() {
				it("should save image with expected annotated os", func() {
					var (
						modifiedOS = "some-os"
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetOS(alpineImageDigest, modifiedOS)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.OS, modifiedOS)
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.OS, modifiedOS)
						}
					}
				})
				it("should save image with expected annotated architecture", func() {
					var (
						modifiedArch = "some-arch"
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetArchitecture(alpineImageDigest, modifiedArch)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.Architecture, modifiedArch)
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.Architecture, modifiedArch)
						}
					}
				})
				it("should save image with expected annotated variant", func() {
					var (
						modifiedVariant = "some-variant"
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetVariant(alpineImageDigest, modifiedVariant)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.Variant, modifiedVariant)
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.Variant, modifiedVariant)
						}
					}
				})
				it("should save image with expected annotated os version", func() {
					var (
						modifiedOSVersion = "some-osVersion"
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetOSVersion(alpineImageDigest, modifiedOSVersion)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.OSVersion, modifiedOSVersion)
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.OSVersion, modifiedOSVersion)
						}
					}
				})
				it("should save image with expected annotated features", func() {
					var (
						modifiedFeatures = []string{"some-features"}
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetFeatures(alpineImageDigest, modifiedFeatures)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.Features, modifiedFeatures)
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.Features, modifiedFeatures)
						}
					}
				})
				it("should save image with expected annotated os features", func() {
					var (
						modifiedOSFeatures = []string{"some-osFeatures"}
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetOSFeatures(alpineImageDigest, modifiedOSFeatures)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.OSFeatures, modifiedOSFeatures)
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Platform.OSFeatures, modifiedOSFeatures)
						}
					}
				})
				it("should save image without annotations", func() {
					var (
						modifiedAnnotations = map[string]string{"some-key": "some-value"}
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetAnnotations(alpineImageDigest, modifiedAnnotations)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Annotations, map[string]string(nil))
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.Annotations, modifiedAnnotations)
						}
					}
				})
				it("should save image with expected annotated urls", func() {
					var (
						modifiedURLs = []string{"some-urls"}
					)

					idx, ok := idx.(*imgutil.Index)
					h.AssertEq(t, ok, true)

					mfest, err := idx.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					for _, m := range mfest.Manifests {
						hash, err := v1.NewHash(alpineImageDigestStr)
						h.AssertNil(t, err)

						if hash == m.Digest {
							continue
						}

						err = idx.Remove(alpineImageDigest.Digest(m.Digest.String()))
						h.AssertNil(t, err)
					}

					err = idx.SetURLs(alpineImageDigest, modifiedURLs)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					format, err := idx.MediaType()
					h.AssertNil(t, err)

					if format == types.DockerManifestList {
						idx, err := local.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.URLs, modifiedURLs)
						}
					} else {
						idx, err := layout.NewIndex(indexName, index.WithXDGRuntimePath(xdgPath))
						h.AssertNil(t, err)

						imgIdx, ok := idx.(*imgutil.Index)
						h.AssertEq(t, ok, true)

						mfest, err := imgIdx.IndexManifest()
						h.AssertNil(t, err)
						h.AssertNotEq(t, mfest, nil)

						for _, m := range mfest.Manifests {
							h.AssertEq(t, m.URLs, modifiedURLs)
						}
					}
				})
				it("should return an error", func() {})
			})
			when("#Push", func() {
				it("should push index to registry", func() {
					err := idx.Push(imgutil.WithInsecure(true))
					h.AssertNil(t, err)
				})
				it("should return an error", func() {})
			})
			when("#Inspect", func() {
				it("should print index raw manifest", func() {
					err := idx.Inspect()
					h.AssertNotEq(t, err, nil)
					h.AssertNotEq(t, errors.Is(err, imgutil.ErrIndexNeedToBeSaved), true)
				})
			})
			when("#Delete", func() {
				it("should delete index from local storage", func() {})
				it("should return an error", func() {
					err := idx.Delete()
					h.AssertNil(t, err)

					err = idx.Delete()
					h.AssertNotEq(t, err, nil)
				})
			})
		})
	})
}
