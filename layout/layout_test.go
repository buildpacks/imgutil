package layout_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/buildpacks/imgutil"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil/layout"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	h "github.com/buildpacks/imgutil/testhelpers"
)

// FIXME: relevant tests in this file should be moved into new_test.go and save_test.go to mirror the implementation
func TestLayout(t *testing.T) {
	spec.Run(t, "Image", testImage, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testImage(t *testing.T, when spec.G, it spec.S) {
	var (
		testImage           v1.Image
		tmpDir              string
		testDataDir         string
		imagePath           string
		fullBaseImagePath   string
		sparseBaseImagePath string
		err                 error
	)

	it.Before(func() {
		// creates a v1.Image from a remote repository
		testImage = h.RemoteRunnableBaseImage(t)

		// creates the directory to save all the OCI images on disk
		tmpDir, err = os.MkdirTemp("", "layout")
		h.AssertNil(t, err)

		// global directory and paths
		testDataDir = filepath.Join("testdata", "layout")
		fullBaseImagePath = filepath.Join(testDataDir, "busybox")
		sparseBaseImagePath = filepath.Join(testDataDir, "busybox-sparse")
	})

	it.After(func() {
		// removes all images created
		os.RemoveAll(tmpDir)
	})

	when("#NewIndex", func() {
		it("should return new Index", func() {

		})
		it("should return an error", func() {

		})
		when("#NewIndex options", func() {
			when("#OS", func() {
				it("should return expected os", func() {})
				it("should return an error", func() {})
			})
			when("#Architecture", func() {
				it("should return expected architecture", func() {})
				it("should return an error", func() {})
			})
			when("#Variant", func() {
				it("should return expected variant", func() {})
				it("should return an error", func() {})
			})
			when("#OSVersion", func() {
				it("should return expected os version", func() {})
				it("should return an error", func() {})
			})
			when("#Features", func() {
				it("should return expected features for image", func() {})
				it("should return expected features for index", func() {})
				it("should return an error", func() {})
			})
			when("#OSFeatures", func() {
				it("should return expected os features for image", func() {})
				it("should return expected os features for index", func() {})
				it("should return an error", func() {})
			})
			when("#Annotations", func() {
				it("should return expected annotations for oci index", func() {})
				it("should return expected annotations for oci image", func() {})
				it("should not return annotations for docker index", func() {})
				it("should not return annotations for docker image", func() {})
				it("should return an error", func() {})
			})
			when("#URLs", func() {
				it("should return expected urls for index", func() {})
				it("should return expected urls for image", func() {})
				it("should return an error", func() {})
			})
			when("#SetOS", func() {
				it("should annotate the image os", func() {})
				it("should return an error", func() {})
			})
			when("#SetArchitecture", func() {
				it("should annotate the image architecture", func() {})
				it("should return an error", func() {})
			})
			when("#SetVariant", func() {
				it("should annotate the image variant", func() {})
				it("should return an error", func() {})
			})
			when("#SetOSVersion", func() {
				it("should annotate the image os version", func() {})
				it("should return an error", func() {})
			})
			when("#SetFeatures", func() {
				it("should annotate the image features", func() {})
				it("should annotate the index features", func() {})
				it("should return an error", func() {})
			})
			when("#SetOSFeatures", func() {
				it("should annotate the image os features", func() {})
				it("should annotate the index os features", func() {})
				it("should return an error", func() {})
			})
			when("#SetAnnotations", func() {
				it("should annotate the image annotations", func() {})
				it("should annotate the index annotations", func() {})
				it("should return an error", func() {})
			})
			when("#SetURLs", func() {
				it("should annotate the image urls", func() {})
				it("should annotate the index urls", func() {})
				it("should return an error", func() {})
			})
			when("#Add", func() {
				it("should add an image", func() {})
				it("should add all images in index", func() {})
				it("should add platform specific image", func() {})
				it("should add target specific image", func() {})
				it("should return an error", func() {})
			})
			when("#Save", func() {
				it("should save image with expected annotated os", func() {})
				it("should save image with expected annotated architecture", func() {})
				it("should save image with expected annotated variant", func() {})
				it("should save image with expected annotated os version", func() {})
				it("should save image with expected annotated features", func() {})
				it("should save image with expected annotated os features", func() {})
				it("should save image with expected annotated annotations for oci", func() {})
				it("should save image without annotations for docker", func() {})
				it("should save image with expected annotated urls", func() {})
				it("should return an error", func() {})
			})
			when("#Push", func() {
				it("should push index to registry", func() {})
				it("should return an error", func() {})
			})
			when("#Inspect", func() {
				it("should return an error", func() {})
				it("should print index raw manifest", func() {})
			})
			when("#Delete", func() {
				it("should delete index from local storage", func() {})
				it("should return an error", func() {})
			})
		})
	})

	when("#NewImage", func() {
		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "new-image")
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("no base image or platform is given", func() {
			it("sets sensible defaults for all required fields", func() {
				// os, architecture, and rootfs are required per https://github.com/opencontainers/image-spec/blob/master/config.md
				img, err := layout.NewImage(imagePath)
				h.AssertNil(t, err)
				h.AssertNil(t, img.Save())

				os, err := img.OS()
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				osVersion, err := img.OSVersion()
				h.AssertNil(t, err)
				h.AssertEq(t, osVersion, "")

				arch, err := img.Architecture()
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				h.AssertOCIMediaTypes(t, img)
			})
		})

		when("#WithDefaultPlatform", func() {
			it("sets all platform required fields for windows", func() {
				img, err := layout.NewImage(
					imagePath,
					layout.WithDefaultPlatform(imgutil.Platform{
						Architecture: "arm",
						OS:           "windows",
						OSVersion:    "10.0.17763.316",
					}),
				)
				h.AssertNil(t, err)
				h.AssertNil(t, img.Save())
				h.AssertOCIMediaTypes(t, img)

				arch, err := img.Architecture()
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm")

				os, err := img.OS()
				h.AssertNil(t, err)
				h.AssertEq(t, os, "windows")

				osVersion, err := img.OSVersion()
				h.AssertNil(t, err)
				h.AssertEq(t, osVersion, "10.0.17763.316")

				_, err = img.TopLayer()
				h.AssertError(t, err, "has no layers")
			})

			it("sets all platform required fields for linux", func() {
				img, err := layout.NewImage(
					imagePath,
					layout.WithDefaultPlatform(imgutil.Platform{
						Architecture: "arm",
						OS:           "linux",
					}),
				)
				h.AssertNil(t, err)
				h.AssertNil(t, img.Save())
				h.AssertOCIMediaTypes(t, img)

				arch, err := img.Architecture()
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm")

				os, err := img.OS()
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				_, err = img.TopLayer()
				h.AssertError(t, err, "has no layers")
			})
		})

		when("#FromBaseImage", func() {
			when("no platform is specified", func() {
				when("base image is provided", func() {
					it.Before(func() {
						var opts []remote.Option
						testImage = h.RemoteImage(t, "arm64v8/busybox@sha256:50edf1d080946c6a76989d1c3b0e753b62f7d9b5f5e66e88bef23ebbd1e9709c", opts)
					})

					it("sets the initial state from a linux/arm base image", func() {
						existingLayerSha := "sha256:5a0b973aa300cd2650869fd76d8546b361fcd6dfc77bd37b9d4f082cca9874e4"

						img, err := layout.NewImage(imagePath, layout.FromBaseImage(testImage), layout.WithMediaTypes(imgutil.OCITypes))
						h.AssertNil(t, err)
						h.AssertOCIMediaTypes(t, img)

						os, err := img.OS()
						h.AssertNil(t, err)
						h.AssertEq(t, os, "linux")

						osVersion, err := img.OSVersion()
						h.AssertNil(t, err)
						h.AssertEq(t, osVersion, "")

						arch, err := img.Architecture()
						h.AssertNil(t, err)
						h.AssertEq(t, arch, "arm64")

						readCloser, err := img.GetLayer(existingLayerSha)
						h.AssertNil(t, err)
						defer readCloser.Close()
					})
				})

				when("base image does not exist", func() {
					it("returns an empty image", func() {
						img, err := layout.NewImage(imagePath, layout.FromBaseImage(nil))
						h.AssertNil(t, err)
						h.AssertOCIMediaTypes(t, img)

						_, err = img.TopLayer()
						h.AssertError(t, err, "has no layers")
					})
				})
			})
		})

		when("#FromBaseImagePath", func() {
			when("base image is full saved on disk", func() {
				it("sets the initial state from the base image", func() {
					existingLayerSha := "sha256:40cf597a9181e86497f4121c604f9f0ab208950a98ca21db883f26b0a548a2eb"

					img, err := layout.NewImage(imagePath, layout.FromBaseImagePath(fullBaseImagePath))
					h.AssertNil(t, err)
					h.AssertDockerMediaTypes(t, img)

					os, err := img.OS()
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					osVersion, err := img.OSVersion()
					h.AssertNil(t, err)
					h.AssertEq(t, osVersion, "")

					arch, err := img.Architecture()
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					readCloser, err := img.GetLayer(existingLayerSha)
					h.AssertNil(t, err)
					defer readCloser.Close()
				})
			})

			when("base image is sparse saved on disk", func() {
				it("sets the initial state from the base image", func() {
					existingLayerSha := "sha256:40cf597a9181e86497f4121c604f9f0ab208950a98ca21db883f26b0a548a2eb"

					img, err := layout.NewImage(imagePath, layout.FromBaseImagePath(sparseBaseImagePath))
					h.AssertNil(t, err)
					h.AssertDockerMediaTypes(t, img)

					os, err := img.OS()
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					osVersion, err := img.OSVersion()
					h.AssertNil(t, err)
					h.AssertEq(t, osVersion, "")

					arch, err := img.Architecture()
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					readCloser, err := img.GetLayer(existingLayerSha)
					h.AssertNil(t, err)
					defer readCloser.Close()
				})
			})

			when("base image does not exist", func() {
				it("returns an empty image", func() {
					img, err := layout.NewImage(imagePath, layout.FromBaseImagePath("some-bad-repo-name"))
					h.AssertNil(t, err)
					h.AssertOCIMediaTypes(t, img)

					_, err = img.TopLayer()
					h.AssertError(t, err, "has no layers")
				})
			})
		})

		when("#WithMediaTypes", func() {
			it("sets the requested media types", func() {
				img, err := layout.NewImage(
					imagePath,
					layout.WithMediaTypes(imgutil.DockerTypes),
				)
				h.AssertNil(t, err)
				h.AssertDockerMediaTypes(t, img) // before saving
				// add a random layer
				path, diffID, _ := h.RandomLayer(t, tmpDir)
				err = img.AddLayerWithDiffID(path, diffID)
				h.AssertNil(t, err)
				h.AssertDockerMediaTypes(t, img) // after adding a layer
				h.AssertNil(t, img.Save())
				h.AssertDockerMediaTypes(t, img) // after saving
			})
		})

		when("#WithPreviousImage", func() {
			var (
				layerDiffID       string
				previousImagePath string
			)

			it.Before(func() {
				// value from testdata/layout/my-previous-image config.RootFS.DiffIDs
				layerDiffID = "sha256:ebc931a4ab83b0c934f2436c975cca387bc1bcebd1a5ced12824ff7592f317ea"
				imagePath = filepath.Join(tmpDir, "save-from-previous-image")
				previousImagePath = filepath.Join(testDataDir, "my-previous-image")
			})

			when("previous image exists", func() {
				when("previous image is not sparse", func() {
					it.Before(func() {
						imagePath = filepath.Join(tmpDir, "save-from-previous-image")
						previousImagePath = filepath.Join(testDataDir, "my-previous-image")
					})

					it("provides reusable layers", func() {
						img, err := layout.NewImage(imagePath, layout.WithPreviousImage(previousImagePath))
						h.AssertNil(t, err)
						h.AssertOCIMediaTypes(t, img)

						h.AssertNil(t, img.ReuseLayer(layerDiffID))
					})
				})

				when("previous image is sparse", func() {
					it.Before(func() {
						imagePath = filepath.Join(tmpDir, "save-from-previous-sparse-image")
						previousImagePath = filepath.Join(testDataDir, "my-previous-sparse-image")
					})

					it("provides reusable layers", func() {
						img, err := layout.NewImage(imagePath, layout.WithPreviousImage(previousImagePath))
						h.AssertNil(t, err)
						h.AssertOCIMediaTypes(t, img)

						h.AssertNil(t, img.ReuseLayer(layerDiffID))
					})
				})
			})

			when("previous image does not exist", func() {
				it("does not error", func() {
					_, err := layout.NewImage(
						imagePath,
						layout.WithPreviousImage("some-bad-repo-name"),
					)

					h.AssertNil(t, err)
				})
			})
		})
	})

	when("#WorkingDir", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "working-dir-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("working dir is saved on disk in OCI layout format", func() {
			image.SetWorkingDir("/temp")

			err := image.Save()
			h.AssertNil(t, err)

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)

			workingDir := configFile.Config.WorkingDir
			h.AssertEq(t, workingDir, "/temp")

			// Let's load the OCI image saved previously
			imageLoaded, err := layout.NewImage(imagePath, layout.FromBaseImagePath(imagePath))
			h.AssertNil(t, err)

			// Let's verify the working directory
			value, err := imageLoaded.WorkingDir()
			h.AssertNil(t, err)
			h.AssertEq(t, value, "/temp")
		})
	})

	when("#EntryPoint", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "entry-point-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("entrypoint added is saved on disk in OCI layout format", func() {
			image.SetEntrypoint("bin/tool")

			err = image.Save()
			h.AssertNil(t, err)

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)

			entryPoints := configFile.Config.Entrypoint
			h.AssertEq(t, len(entryPoints), 1)
			h.AssertEq(t, entryPoints[0], "bin/tool")

			// Let's load the OCI image saved previously
			imageLoaded, err := layout.NewImage(imagePath, layout.FromBaseImagePath(imagePath))
			h.AssertNil(t, err)

			// Let's verify the working directory
			values, err := imageLoaded.Entrypoint()
			h.AssertNil(t, err)
			h.AssertEq(t, len(values), 1)
			h.AssertEq(t, values[0], "bin/tool")
		})
	})

	when("#Labels", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "labels-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("label added is saved on disk in OCI layout format", func() {
			image.SetLabel("foo", "bar")

			err := image.Save()
			h.AssertNil(t, err)

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)

			labels := configFile.Config.Labels
			h.AssertEq(t, len(labels), 1)
			h.AssertEq(t, labels["foo"], "bar")

			// Let's load the OCI image saved previously
			imageLoaded, err := layout.NewImage(imagePath, layout.FromBaseImagePath(imagePath))
			h.AssertNil(t, err)

			// Labels
			labelsLoaded, err := imageLoaded.Labels()
			h.AssertNil(t, err)
			h.AssertEq(t, labelsLoaded["foo"], "bar")

			// Let's validate we can recover the label value
			value, err := imageLoaded.Label("foo")
			h.AssertNil(t, err)
			h.AssertEq(t, value, "bar")

			// Remove label
			err = imageLoaded.RemoveLabel("foo")
			h.AssertNil(t, err)

			err = imageLoaded.Save()
			h.AssertNil(t, err)

			_, configFile = h.ReadManifestAndConfigFile(t, imagePath)

			labels = configFile.Config.Labels
			h.AssertEq(t, len(labels), 0)
		})
	})

	when("#Env", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "env-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("environment variable added is saved on disk in OCI layout format", func() {
			image.SetEnv("FOO_KEY", "bar")

			err := image.Save()
			h.AssertNil(t, err)

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)

			envs := configFile.Config.Env
			h.AssertEq(t, len(envs), 1)
			h.AssertEq(t, envs[0], "FOO_KEY=bar")

			// Let's load the OCI image saved previously
			imageLoaded, err := layout.NewImage(imagePath, layout.FromBaseImagePath(imagePath))
			h.AssertNil(t, err)

			// Let's verify the environment variable
			value, err := imageLoaded.Env("FOO_KEY")
			h.AssertNil(t, err)
			h.AssertEq(t, value, "bar")
		})
	})

	when("#Name", func() {
		it("always returns the original name", func() {
			img, err := layout.NewImage(imagePath)
			h.AssertNil(t, err)
			h.AssertEq(t, img.Name(), imagePath)
		})
	})

	when("#CreatedAt", func() {
		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "new-created-at-image")
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("returns the containers created at time", func() {
			img, err := layout.NewImage(imagePath, layout.FromBaseImagePath(fullBaseImagePath))
			h.AssertNil(t, err)

			expectedTime := time.Date(2022, 11, 18, 1, 19, 29, 442257773, time.UTC)

			createdTime, err := img.CreatedAt()

			h.AssertNil(t, err)
			h.AssertEq(t, createdTime, expectedTime)
		})
	})

	when("#SetLabel", func() {
		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "new-set-label-image")
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("image exists", func() {
			it("sets label on img object", func() {
				img, err := layout.NewImage(imagePath)
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("mykey", "new-val"))
				label, err := img.Label("mykey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "new-val")
			})

			it("saves label", func() {
				img, err := layout.NewImage(imagePath)
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("mykey", "new-val"))

				h.AssertNil(t, img.Save())

				testImgPath := filepath.Join(tmpDir, "new-test-image")
				testImg, err := layout.NewImage(
					testImgPath,
					layout.FromBaseImage(img),
				)
				h.AssertNil(t, err)

				layoutLabel, err := testImg.Label("mykey")
				h.AssertNil(t, err)

				h.AssertEq(t, layoutLabel, "new-val")
			})
		})
	})

	when("#RemoveLabel", func() {
		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "new-remove-label-image")
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("image exists", func() {
			var baseImageNamePath = filepath.Join(tmpDir, "my-base-image")

			it.Before(func() {
				baseImage, err := layout.NewImage(baseImageNamePath, layout.FromBaseImage(testImage))
				h.AssertNil(t, err)
				h.AssertNil(t, baseImage.SetLabel("custom.label", "new-val"))
				h.AssertNil(t, baseImage.Save())
			})

			it.After(func() {
				os.RemoveAll(baseImageNamePath)
			})

			it("removes label on img object", func() {
				img, err := layout.NewImage(imagePath, layout.FromBaseImagePath(baseImageNamePath))
				h.AssertNil(t, err)

				h.AssertNil(t, img.RemoveLabel("custom.label"))

				labels, err := img.Labels()
				h.AssertNil(t, err)
				_, exists := labels["my.custom.label"]
				h.AssertEq(t, exists, false)
			})

			it("saves removal of label", func() {
				img, err := layout.NewImage(imagePath, layout.FromBaseImagePath(baseImageNamePath))
				h.AssertNil(t, err)

				h.AssertNil(t, img.RemoveLabel("custom.label"))
				h.AssertNil(t, img.Save())

				testImgPath := filepath.Join(tmpDir, "new-test-image")
				testImg, err := layout.NewImage(
					testImgPath,
					layout.FromBaseImage(img),
				)
				h.AssertNil(t, err)

				layoutLabel, err := testImg.Label("custom.label")
				h.AssertNil(t, err)
				h.AssertEq(t, layoutLabel, "")
			})
		})
	})

	when("#SetCmd", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "set-cmd-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("CMD is added and saved on disk in OCI layout format", func() {
			image.SetCmd("echo", "Hello World")

			err = image.Save()
			h.AssertNil(t, err)

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)

			cmds := configFile.Config.Cmd
			h.AssertEq(t, len(cmds), 2)
			h.AssertEq(t, cmds[0], "echo")
			h.AssertEq(t, cmds[1], "Hello World")
		})
	})

	when("#TopLayer", func() {
		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "top-layer-from-base-image-path")
		})
		it.After(func() {
			os.RemoveAll(imagePath)
		})
		when("sparse image was saved on disk in OCI layout format", func() {
			it("Top layer DiffID from base image", func() {
				image, err := layout.NewImage(imagePath, layout.FromBaseImagePath(sparseBaseImagePath))
				h.AssertNil(t, err)

				diffID, err := image.TopLayer()
				h.AssertNil(t, err)

				// from testdata/layout/busybox-sparse/
				expectedDiffID := "sha256:40cf597a9181e86497f4121c604f9f0ab208950a98ca21db883f26b0a548a2eb"
				h.AssertEq(t, diffID, expectedDiffID)
			})
		})
	})

	when("#Save", func() {
		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("#FromBaseImage with full image", func() {
			it.Before(func() {
				imagePath = filepath.Join(tmpDir, "save-from-base-image")
			})

			when("additional names are provided", func() {
				it("creates an image and save it to both path provided", func() {
					image, err := layout.NewImage(imagePath, layout.FromBaseImage(testImage))
					h.AssertNil(t, err)

					anotherPath := filepath.Join(tmpDir, "another-save-from-base-image")
					// save on disk in OCI
					err = image.Save(anotherPath)
					h.AssertNil(t, err)

					//  expected blobs: manifest, config, layer
					h.AssertBlobsLen(t, imagePath, 3)
					index := h.ReadIndexManifest(t, imagePath)
					h.AssertEq(t, len(index.Manifests), 1)

					// assert image saved on additional path
					h.AssertBlobsLen(t, anotherPath, 3)
					index = h.ReadIndexManifest(t, anotherPath)
					h.AssertEq(t, len(index.Manifests), 1)
				})
			})

			when("no additional names are provided", func() {
				it("creates an image with all the layers from the underlying image", func() {
					image, err := layout.NewImage(imagePath, layout.FromBaseImage(testImage))
					h.AssertNil(t, err)

					// save on disk in OCI
					err = image.Save()
					h.AssertNil(t, err)

					//  expected blobs: manifest, config, layer
					h.AssertBlobsLen(t, imagePath, 3)

					// assert additional name
					index := h.ReadIndexManifest(t, imagePath)
					h.AssertEq(t, len(index.Manifests), 1)
					h.AssertEq(t, 0, len(index.Manifests[0].Annotations))
				})
			})
		})

		when("#FromBaseImagePath", func() {
			it.Before(func() {
				imagePath = filepath.Join(tmpDir, "save-from-base-image-path")
			})

			when("full image was saved on disk in OCI layout format", func() {
				when("a new layer was added", func() {
					it("image is saved on disk with all the layers", func() {
						image, err := layout.NewImage(
							imagePath,
							layout.FromBaseImagePath(fullBaseImagePath),
							layout.WithHistory(),
						)
						h.AssertNil(t, err)

						// add a random layer
						path1, diffID1, _ := h.RandomLayer(t, tmpDir)
						h.AssertNil(t, image.AddLayerWithDiffID(path1, diffID1))

						// add a layer with history
						path2, diffID2, _ := h.RandomLayer(t, tmpDir)
						h.AssertNil(t, image.AddLayerWithDiffIDAndHistory(path2, diffID2, v1.History{CreatedBy: "some-history"}))

						// save on disk in OCI
						image.AnnotateRefName("latest")
						h.AssertNil(t, image.Save())

						// expected blobs: manifest, config, base image layer, new random layer, new layer with history
						h.AssertBlobsLen(t, imagePath, 5)

						// assert additional name
						index := h.ReadIndexManifest(t, imagePath)
						h.AssertEq(t, len(index.Manifests), 1)
						h.AssertEq(t, 1, len(index.Manifests[0].Annotations))
						h.AssertEqAnnotation(t, index.Manifests[0], layout.ImageRefNameKey, "latest")

						// assert history
						digest := index.Manifests[0].Digest
						manifest := h.ReadManifest(t, digest, imagePath)
						config := h.ReadConfigFile(t, manifest, imagePath)
						h.AssertEq(t, len(config.History), 3)
						lastLayerHistory := config.History[len(config.History)-1]
						h.AssertEq(t, lastLayerHistory, v1.History{
							Created:   v1.Time{Time: imgutil.NormalizedDateTime},
							CreatedBy: "some-history",
						})
					})
				})
			})

			when("sparse image was saved on disk in OCI layout format", func() {
				when("a new layer was added", func() {
					it("image is saved on disk with the new layer only", func() {
						image, err := layout.NewImage(imagePath, layout.FromBaseImagePath(sparseBaseImagePath))
						h.AssertNil(t, err)

						// add a random layer
						path, diffID, _ := h.RandomLayer(t, tmpDir)
						err = image.AddLayerWithDiffID(path, diffID)
						h.AssertNil(t, err)

						// adds org.opencontainers.image.ref.name annotation
						image.AnnotateRefName("latest")

						// save on disk in OCI
						err = image.Save()
						h.AssertNil(t, err)

						// expected blobs: manifest, config, new random layer
						h.AssertBlobsLen(t, imagePath, 3)

						// assert org.opencontainers.image.ref.name annotation
						index := h.ReadIndexManifest(t, imagePath)
						h.AssertEq(t, len(index.Manifests), 1)
						h.AssertEq(t, 1, len(index.Manifests[0].Annotations))
						h.AssertEqAnnotation(t, index.Manifests[0], layout.ImageRefNameKey, "latest")
					})
				})
			})
		})

		when("#FromPreviousImage", func() {
			var (
				prevImageLayerDiffID string
				previousImagePath    string
			)

			it.Before(func() {
				// value from testdata/layout/my-previous-image config.RootFS.DiffIDs
				prevImageLayerDiffID = "sha256:ebc931a4ab83b0c934f2436c975cca387bc1bcebd1a5ced12824ff7592f317ea"
				imagePath = filepath.Join(tmpDir, "save-from-previous-image")
				previousImagePath = filepath.Join(testDataDir, "my-previous-image")
			})

			it.After(func() {
				os.RemoveAll(imagePath)
			})

			when("previous image is not sparse", func() {
				it.Before(func() {
					imagePath = filepath.Join(tmpDir, "save-from-previous-image")
					previousImagePath = filepath.Join(testDataDir, "my-previous-image")
				})

				when("#ReuseLayer", func() {
					it("it reuses layer from previous image", func() {
						image, err := layout.NewImage(imagePath, layout.WithPreviousImage(previousImagePath))
						h.AssertNil(t, err)

						// reuse layer from previous image
						err = image.ReuseLayer(prevImageLayerDiffID)
						h.AssertNil(t, err)

						// save on disk in OCI
						err = image.Save()
						h.AssertNil(t, err)

						// expected blobs: manifest, config, reuse random layer
						h.AssertBlobsLen(t, imagePath, 3)

						mediaType, err := image.MediaType()
						h.AssertNil(t, err)
						h.AssertEq(t, mediaType, types.OCIManifestSchema1)
					})

					when("there is history", func() {
						var prevHistory []v1.History

						it.Before(func() {
							prevImage, err := layout.NewImage(
								filepath.Join(tmpDir, "previous-with-history"),
								layout.FromBaseImagePath(previousImagePath),
								layout.WithHistory(),
							)
							h.AssertNil(t, err)
							// set history
							layers, err := prevImage.Image.Layers()
							h.AssertNil(t, err)
							prevHistory = make([]v1.History, len(layers))
							for idx := range prevHistory {
								prevHistory[idx].CreatedBy = fmt.Sprintf("some-history-%d", idx)
							}
							h.AssertNil(t, prevImage.SetHistory(prevHistory))
							h.AssertNil(t, prevImage.Save())
						})

						it("reuses a layer with history", func() {
							img, err := layout.NewImage(
								imagePath,
								layout.WithPreviousImage(filepath.Join(tmpDir, "previous-with-history")),
								layout.WithHistory(),
							)
							h.AssertNil(t, err)

							// add a layer
							newBaseLayerPath, _, _ := h.RandomLayer(t, tmpDir)
							h.AssertNil(t, err)
							defer os.Remove(newBaseLayerPath)
							h.AssertNil(t, img.AddLayer(newBaseLayerPath))

							// re-use a layer
							h.AssertNil(t, img.ReuseLayer(prevImageLayerDiffID))

							h.AssertNil(t, img.Save())

							layers, err := img.Image.Layers()
							h.AssertNil(t, err)

							// get re-used layer
							reusedLayer := layers[len(layers)-1]
							reusedLayerSHA, err := reusedLayer.DiffID()
							h.AssertNil(t, err)
							h.AssertEq(t, reusedLayerSHA.String(), prevImageLayerDiffID)

							history, err := img.History()
							h.AssertNil(t, err)
							h.AssertEq(t, len(history), len(layers))
							h.AssertEq(t, len(history) >= 2, true)

							// check re-used layer history
							reusedLayerHistory := history[len(history)-1]
							h.AssertEq(t, strings.Contains(reusedLayerHistory.CreatedBy, "some-history-"), true)

							// check added layer history
							addedLayerHistory := history[len(history)-2]
							h.AssertEq(t, addedLayerHistory, v1.History{Created: v1.Time{Time: imgutil.NormalizedDateTime}})
						})
					})
				})

				when("#ReuseLayerWithHistory", func() {
					it.Before(func() {
						prevImage, err := layout.NewImage(
							filepath.Join(tmpDir, "previous-with-history"),
							layout.FromBaseImagePath(previousImagePath),
							layout.WithHistory(),
						)
						h.AssertNil(t, err)
						h.AssertNil(t, prevImage.Save())
					})

					it("reuses a layer with history", func() {
						img, err := layout.NewImage(
							imagePath,
							layout.WithPreviousImage(filepath.Join(tmpDir, "previous-with-history")),
							layout.WithHistory(),
						)
						h.AssertNil(t, err)

						// add a layer
						newBaseLayerPath, _, _ := h.RandomLayer(t, tmpDir)
						h.AssertNil(t, err)
						defer os.Remove(newBaseLayerPath)
						h.AssertNil(t, img.AddLayer(newBaseLayerPath))

						// re-use a layer
						h.AssertNil(t, img.ReuseLayerWithHistory(prevImageLayerDiffID, v1.History{CreatedBy: "some-new-history"}))

						h.AssertNil(t, img.Save())

						layers, err := img.Image.Layers()
						h.AssertNil(t, err)

						// get re-used layer
						reusedLayer := layers[len(layers)-1]
						reusedLayerSHA, err := reusedLayer.DiffID()
						h.AssertNil(t, err)
						h.AssertEq(t, reusedLayerSHA.String(), prevImageLayerDiffID)

						history, err := img.History()
						h.AssertNil(t, err)
						h.AssertEq(t, len(history), len(layers))
						h.AssertEq(t, len(history) >= 2, true)

						// check re-used layer history
						reusedLayerHistory := history[len(history)-1]
						h.AssertEq(t, strings.Contains(reusedLayerHistory.CreatedBy, "some-new-history"), true)

						// check added layer history
						addedLayerHistory := history[len(history)-2]
						h.AssertEq(t, addedLayerHistory, v1.History{Created: v1.Time{Time: imgutil.NormalizedDateTime}})
					})
				})
			})

			when("previous image is sparse", func() {
				it.Before(func() {
					imagePath = filepath.Join(tmpDir, "save-from-previous-sparse-image")
					previousImagePath = filepath.Join(testDataDir, "my-previous-sparse-image")
				})

				when("#ReuseLayer", func() {
					it("it reuses layer from previous image", func() {
						image, err := layout.NewImage(imagePath, layout.WithPreviousImage(previousImagePath))
						h.AssertNil(t, err)

						// reuse layer from previous image
						err = image.ReuseLayer(prevImageLayerDiffID)
						h.AssertNil(t, err)

						// save on disk in OCI
						err = image.Save()
						h.AssertNil(t, err)

						// expected blobs: manifest, config, random layer is not present
						h.AssertBlobsLen(t, imagePath, 2)

						// assert it has the reused layer
						index := h.ReadIndexManifest(t, imagePath)
						manifest := h.ReadManifest(t, index.Manifests[0].Digest, imagePath)
						h.AssertEq(t, len(manifest.Layers), 1)
					})
				})
			})
		})
	})

	when("#Found", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "found-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("image doesn't exist on disk", func() {
			it.Before(func() {
				imagePath = filepath.Join(tmpDir, "non-exist-image")
				image, err = layout.NewImage(imagePath)
				h.AssertNil(t, err)
			})

			it("returns false", func() {
				h.AssertTrue(t, func() bool {
					return !image.Found()
				})
			})
		})

		when("image exists on disk", func() {
			it.Before(func() {
				imagePath = filepath.Join(testDataDir, "my-previous-image")
				image, err = layout.NewImage(imagePath)
				h.AssertNil(t, err)
			})

			it.After(func() {
				// We don't want to delete testdata/my-previous-image
				imagePath = ""
			})

			it("returns true", func() {
				h.AssertTrue(t, func() bool {
					return image.Found()
				})
			})
		})
	})

	when("#Valid", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "found-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("image doesn't exist on disk", func() {
			it.Before(func() {
				imagePath = filepath.Join(tmpDir, "non-exist-image")
				image, err = layout.NewImage(imagePath)
				h.AssertNil(t, err)
			})

			it("returns false", func() {
				h.AssertTrue(t, func() bool {
					return !image.Found()
				})
			})
		})

		when("image exists on disk", func() {
			it.Before(func() {
				imagePath = filepath.Join(testDataDir, "my-previous-image")
				image, err = layout.NewImage(imagePath)
				h.AssertNil(t, err)
			})

			it.After(func() {
				// We don't want to delete testdata/my-previous-image
				imagePath = ""
			})

			it("returns true", func() {
				h.AssertTrue(t, func() bool {
					return image.Found()
				})
			})
		})
	})

	when("#Delete", func() {
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "delete-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)

			// Write the image on disk
			err = image.Save()
			h.AssertNil(t, err)

			// Image must be found
			h.AssertTrue(t, func() bool {
				return image.Found()
			})
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("images is deleted from disk", func() {
			err = image.Delete()
			h.AssertNil(t, err)

			// Image must not be found anymore
			h.AssertTrue(t, func() bool {
				return !image.Found()
			})
		})
	})

	when("#Platform", func() {
		var platform imgutil.Platform
		var image *layout.Image

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "feature-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)

			platform = imgutil.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "5678",
			}
		})

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it("Platform values are saved on disk in OCI layout format", func() {
			image.SetArchitecture("amd64")
			image.SetOS("linux")
			image.SetOSVersion("1234")

			image.Save()

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)
			h.AssertEq(t, configFile.OS, "linux")
			h.AssertEq(t, configFile.Architecture, "amd64")
			h.AssertEq(t, configFile.OSVersion, "1234")
		})

		it("Default Platform values are saved on disk in OCI layout format", func() {
			image, err = layout.NewImage(imagePath, layout.WithDefaultPlatform(platform))
			h.AssertNil(t, err)

			image.Save()

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)
			h.AssertEq(t, configFile.OS, "linux")
			h.AssertEq(t, configFile.Architecture, "amd64")
			h.AssertEq(t, configFile.OSVersion, "5678")
		})
	})

	when("#GetLayer", func() {
		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "get-layer-from-base-image-path")
		})
		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("sparse image was saved on disk in OCI layout format", func() {
			it("Get layer from sparse base image", func() {
				image, err := layout.NewImage(imagePath, layout.FromBaseImagePath(sparseBaseImagePath))
				h.AssertNil(t, err)
				// from testdata/layout/busybox-sparse/
				diffID := "sha256:40cf597a9181e86497f4121c604f9f0ab208950a98ca21db883f26b0a548a2eb"
				_, err = image.GetLayer(diffID)
				h.AssertNil(t, err)
			})
		})
	})
}
