package imgutil_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/local"
	"github.com/buildpacks/imgutil/remote"

	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestIndex(t *testing.T) {
	spec.Run(t, "Index", testIndex, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testIndex(t *testing.T, when spec.G, it spec.S) {
	when("getters", func() {
		const (
			indexRefStr                = "grafana/grafana:9.5.15-ubuntu"
			digestRefStr               = "grafana/grafana@sha256:72aa8b5efb5d13a0a604ea7e48a43d1cf5f7db2ca657ed3d108ada21e84e4202"
			imageOS                    = "linux"
			imageArch                  = "arm64"
			xdgPath                    = "xdgPath"
			dockageAlpineIndex         = "dockage/alpine:3.15"
			dockageAlpineDigest        = "dockage/alpine@sha256:15189c4d42ff0bbaab56591d2f32932c45bf2f5fa16031c0430aba64c6688b94"
			dockageAlpineDigestOS      = "linux"
			dockageAlpineDigestArch    = "arm"
			dockageAlpineDigestVariant = "v6"
		)
		when("#OS", func() {
			var idx imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := local.NewIndex(indexRefStr, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				idx = (*imgIdx)
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, indexRefStr))
				h.AssertNil(t, err)
			})
			it("should return os", func() {
				digest, err := name.NewDigest(digestRefStr, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				os, err := idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, imageOS)
			})
			it("should return empty string", func() {
				digest, err := name.NewDigest(digestRefStr, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetOS(digest, "")
				h.AssertNil(t, err)

				os, err := idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "")
			})
		})
		when("#Architecture", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := remote.NewIndex(indexRefStr, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, indexRefStr))
				h.AssertNil(t, err)
			})
			it("should return architecture", func() {
				digest, err := name.NewDigest(digestRefStr, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				arch, err := idx.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, imageArch)
			})
			it("should return empty string", func() {
				digest, err := name.NewDigest(digestRefStr, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetArchitecture(digest, "")
				h.AssertNil(t, err)

				arch, err := idx.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "")
			})
		})
		when("#Variant", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := remote.NewIndex(dockageAlpineIndex, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, dockageAlpineIndex))
				h.AssertNil(t, err)
			})
			it("should return variant", func() {
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				variant, err := idx.Variant(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, dockageAlpineDigestVariant)
			})
			it("should return empty string", func() {
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetVariant(digest, "")
				h.AssertNil(t, err)

				variant, err := idx.Variant(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, dockageAlpineDigestVariant)
			})
		})
		when("#OSVersion", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := remote.NewIndex(dockageAlpineIndex, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, dockageAlpineIndex))
				h.AssertNil(t, err)
			})
			it("should return os version", func() {
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetOSVersion(digest, "0")
				h.AssertNil(t, err)

				version, err := idx.OSVersion(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, version, "0")
			})
			it("should return empty string", func() {
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				version, err := idx.OSVersion(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, version, "")
			})
		})
		when("#Features", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := remote.NewIndex(dockageAlpineIndex, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, dockageAlpineIndex))
				h.AssertNil(t, err)
			})
			it("should return features", func() {
				var feature = []string{"feature1", "feature2"}
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetFeatures(digest, feature)
				h.AssertNil(t, err)

				features, err := idx.Features(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, feature, features)
			})
			it("should return empty slice", func() {
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				features, err := idx.Features(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, features, []string{})
			})
		})
		when("#OSFeatures", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := remote.NewIndex(dockageAlpineIndex, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, dockageAlpineIndex))
				h.AssertNil(t, err)
			})
			it("should return os-features", func() {
				var osFeature = []string{"feature1", "feature2"}
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetOSFeatures(digest, osFeature)
				h.AssertNil(t, err)

				features, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, osFeature, features)
			})
			it("should return empty slice", func() {
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				features, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, features, []string{})
			})
		})
		when("#Annotations", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := remote.NewIndex(dockageAlpineIndex, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, dockageAlpineIndex))
				h.AssertNil(t, err)
			})
			it("should return annotations", func() {
				var annotations = map[string]string{"annotation1": "annotation2"}
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetAnnotations(digest, annotations)
				h.AssertNil(t, err)

				annos, err := idx.Annotations(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, annos, annotations)
			})
			it("should return empty map when format is not oci", func() {
				var idx *imgutil.ImageIndex
				it.Before(func() {
					imgIdx, err := remote.NewIndex(dockageAlpineIndex, true, imgutil.WithXDGRuntimePath(xdgPath), imgutil.WithMediaTypes(imgutil.DockerTypes))
					h.AssertNil(t, err)
					idx = imgIdx
				})
				it.After(func() {
					err := os.RemoveAll(filepath.Join(xdgPath, dockageAlpineIndex))
					h.AssertNil(t, err)
				})
				var annotations = map[string]string{"annotation1": "annotation2"}
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetAnnotations(digest, annotations)
				h.AssertNil(t, err)

				annos, err := idx.Annotations(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, annos, map[string]string{})
			})
			it("should return empty map", func() {
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				annos, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, annos, map[string]string{})
			})
		})
		when("#URLs", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := remote.NewIndex(dockageAlpineIndex, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, dockageAlpineIndex))
				h.AssertNil(t, err)
			})
			it("should return urls", func() {
				var urls = []string{"url1", "url2"}
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetOSFeatures(digest, urls)
				h.AssertNil(t, err)

				url, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, url, urls)
			})
			it("should return empty slice", func() {
				var urls = []string{}
				digest, err := name.NewDigest(dockageAlpineDigest, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.SetOSFeatures(digest, urls)
				h.AssertNil(t, err)

				url, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, url, urls)
			})
		})
	})
	when("misc", func() {
		const (
			xdgPath      = "/xdgPath"
			indexName    = "cncb/sample-image-index"
			indexImage   = "dockage/alpine:3.15"
			image        = "dockage/alpine@sha256:15189c4d42ff0bbaab56591d2f32932c45bf2f5fa16031c0430aba64c6688b94"
			imageOS      = "linux"
			imageArch    = "arm"
			imageVariant = "v6"
		)
		when("#Add", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := local.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, indexName))
				h.AssertNil(t, err)
			})
			it("should add given image", func() {
				ref, err := name.ParseReference(image, name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				err = idx.Add(ref)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(image)
				h.AssertNil(t, err)

				os, err := idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, imageOS)
			})
			when("Index requested to add", func() {
				it("should add platform specific image", func() {
					ref, err := name.ParseReference(indexImage, name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					err = idx.Add(ref)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					path, err := layout.FromPath(filepath.Join(xdgPath, indexName))
					h.AssertNil(t, err)

					ii, err := path.ImageIndex()
					h.AssertNil(t, err)

					im, err := ii.IndexManifest()
					h.AssertNil(t, err)

					var found bool
					for _, manifest := range im.Manifests {
						if manifest.Platform.OS == runtime.GOOS {
							found = true
							break
						}
					}
					h.AssertEq(t, found, true)
				})
				it("should add all images in index", func() {
					ref, err := name.ParseReference(indexImage, name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					err = idx.Add(ref)
					h.AssertNil(t, err)

					err = idx.Save()
					h.AssertNil(t, err)

					path, err := layout.FromPath(filepath.Join(xdgPath, indexName))
					h.AssertNil(t, err)

					ii, err := path.ImageIndex()
					h.AssertNil(t, err)

					im, err := ii.IndexManifest()
					h.AssertNil(t, err)

					h.AssertNotEq(t, len(im.Manifests), 0)
				})
				it("should add image with given OS", func() {
					ref, err := name.ParseReference(indexImage, name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					err = idx.Add(ref, imgutil.WithOS(imageOS))
					h.AssertNil(t, err)

					digest, err := name.NewDigest(image, name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					_, err = idx.OS(digest)
					h.AssertNil(t, err)
				})
				it("should add image with the given Arch", func() {
					ref, err := name.ParseReference(indexImage, name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					err = idx.Add(ref, imgutil.WithArch(imageArch))
					h.AssertNil(t, err)

					digest, err := name.NewDigest(image, name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					_, err = idx.OS(digest)
					h.AssertNil(t, err)
				})
			})
		})
		when("#Save", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := local.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, indexName))
				h.AssertNil(t, err)
			})
			it("should save the image", func() {
				err := idx.Save()
				h.AssertNil(t, err)

				_, err = imgutil.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
			})
			it("should return an error", func() {
				err := idx.Save()
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNotEq(t, err, nil)

				_, err = imgutil.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
			})
			when("modify IndexType", func() {
				var idx *imgutil.ImageIndex
				var digest name.Digest
				var annotations = map[string]string{"annotation": "value"}
				const (
					dockageAlpineDigest = "dockage/alpine@sha256:15189c4d42ff0bbaab56591d2f32932c45bf2f5fa16031c0430aba64c6688b94"
				)
				it.Before(func() {
					imgIdx, err := local.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath), imgutil.WithMediaTypes(imgutil.DockerTypes))
					h.AssertNil(t, err)

					ref, err := name.NewDigest(dockageAlpineDigest, name.Insecure, name.WeakValidation)
					h.AssertNil(t, err)

					digest = ref
					err = imgIdx.Add(ref)
					h.AssertNil(t, err)

					imgIdx.SetAnnotations(ref, annotations)
					idx = imgIdx
				})
				it.After(func() {
					err := os.RemoveAll(filepath.Join(xdgPath, indexName))
					h.AssertNil(t, err)
				})
				it("should not have annotaioins for docker manifest list", func() {
					err := idx.Save()
					h.AssertNil(t, err)

					idx, err = imgutil.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath), imgutil.WithMediaTypes(imgutil.DockerTypes))
					h.AssertNil(t, err)

					annos, err := idx.Annotations(digest)
					h.AssertNil(t, err)

					h.AssertEq(t, annos, map[string]string{})
				})
				it("should save index with correct format", func() {
					err := idx.Save()
					h.AssertNil(t, err)

					path, err := layout.FromPath(filepath.Join(xdgPath, indexName))
					h.AssertNil(t, err)

					ii, err := path.ImageIndex()
					h.AssertNil(t, err)

					im, err := ii.IndexManifest()
					h.AssertNil(t, err)
					h.AssertEq(t, im.MediaType, types.DockerManifestList)
				})
			})
		})
		// when("#Push", func() {
		// 	it("should return an error", func() {})
		// 	it("should push index to secure registry", func() {})
		// 	it("should not push index to insecure registry", func() {})
		// 	it("should push index to insecure registry", func() {})
		// 	it("should change format and push index", func() {})
		// 	it("should delete index from local storage", func() {})
		// })
		when("#Inspect", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := local.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, indexName))
				h.AssertNil(t, err)
			})
			it("should return an error", func() {
				err := idx.Inspect()
				h.AssertNotEq(t, err, nil)
			})
			it("should print index content", func() {
				err := idx.Save()
				h.AssertNil(t, err)

				err = idx.Inspect()
				h.AssertNotEq(t, err.Error(), "{}")
			})
		})
		when("#Remove", func() {
			var digest name.Digest
			const image = "dockage/alpine@sha256:15189c4d42ff0bbaab56591d2f32932c45bf2f5fa16031c0430aba64c6688b94"
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := local.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.NewDigest(image, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				digest = ref
				err = idx.Add(ref)
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, indexName))
				h.AssertNil(t, err)
			})
			it("should remove given image", func() {
				path, err := layout.FromPath(filepath.Join(xdgPath, indexName))
				h.AssertNil(t, err)

				ii, err := path.ImageIndex()
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				_, err = ii.Image(hash)
				h.AssertNil(t, err)

				err = idx.Remove(digest)
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				ii, err = path.ImageIndex()
				h.AssertNil(t, err)

				_, err = ii.Image(hash)
				h.AssertNotEq(t, err, nil)
			})
			it("should return an error", func() {
				path, err := layout.FromPath(filepath.Join(xdgPath, indexName))
				h.AssertNil(t, err)

				ii, err := path.ImageIndex()
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				_, err = ii.Image(hash)
				h.AssertNil(t, err)

				err = idx.Remove(digest)
				h.AssertNil(t, err)

				err = idx.Remove(digest)
				h.AssertNotEq(t, err, nil)
			})
		})
		when("#Delete", func() {
			var idx *imgutil.ImageIndex
			it.Before(func() {
				imgIdx, err := local.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)
				idx = imgIdx
			})
			it.After(func() {
				err := os.RemoveAll(filepath.Join(xdgPath, indexName))
				h.AssertNil(t, err)
			})
			it("should delete given index", func() {
				_, err := imgutil.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				err = idx.Delete()
				h.AssertNil(t, err)

				_, err = imgutil.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
				h.AssertNotEq(t, err, nil)
			})
		})
	})
	when("#NewIndex", func() {
		const (
			xdgPath   = "/xdgPath"
			indexName = "cncb/sample-image-index"
		)
		var idx *imgutil.ImageIndex
		it.Before(func() {
			imgIdx, err := local.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
			h.AssertNil(t, err)

			err = idx.Save()
			h.AssertNil(t, err)
			idx = imgIdx
		})
		it.After(func() {
			err := os.RemoveAll(filepath.Join(xdgPath, indexName))
			h.AssertNil(t, err)
		})
		it("should load index", func() {
			_, err := imgutil.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
			h.AssertNil(t, err)
		})
		it("should return an error", func() {
			err := idx.Delete()
			h.AssertNil(t, err)

			_, err = imgutil.NewIndex(indexName, true, imgutil.WithXDGRuntimePath(xdgPath))
			h.AssertNotEq(t, err, nil)
		})
	})
}
