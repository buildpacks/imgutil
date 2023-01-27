package layout_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/buildpacks/imgutil"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil/layout"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	h "github.com/buildpacks/imgutil/testhelpers"
)

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

						img, err := layout.NewImage(imagePath, layout.FromBaseImage(testImage))
						h.AssertNil(t, err)

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

					_, err = img.TopLayer()
					h.AssertError(t, err, "has no layers")
				})
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
				it("creates an image and adds annotation org.opencontainers.image.ref.name with name provided", func() {
					image, err := layout.NewImage(imagePath, layout.FromBaseImage(testImage))
					h.AssertNil(t, err)

					// save on disk in OCI
					err = image.Save("my-additional-tag")
					h.AssertNil(t, err)

					//  expected blobs: manifest, config, layer
					h.AssertBlobsLen(t, imagePath, 3)

					// assert additional name
					index := h.ReadIndexManifest(t, imagePath)
					h.AssertEq(t, len(index.Manifests), 1)
					h.AssertEq(t, 1, len(index.Manifests[0].Annotations))
					h.AssertEqAnnotation(t, index.Manifests[0], layout.ImageRefNameKey, "my-additional-tag")
				})

				it("creates an image and adds annotation org.opencontainers.image.ref.name with names provided", func() {
					image, err := layout.NewImage(imagePath, layout.FromBaseImage(testImage))
					h.AssertNil(t, err)

					// save on disk in OCI
					err = image.Save("v1.0.0-vendor", "v2.0.0-debug")
					h.AssertNil(t, err)

					//  expected blobs: manifest, config, layer
					h.AssertBlobsLen(t, imagePath, 3)

					// assert additional name
					index := h.ReadIndexManifest(t, imagePath)
					h.AssertEq(t, len(index.Manifests), 1)
					h.AssertEq(t, 1, len(index.Manifests[0].Annotations))
					h.AssertEqAnnotation(t, index.Manifests[0], layout.ImageRefNameKey, "v1.0.0-vendor,v2.0.0-debug")
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
						image, err := layout.NewImage(imagePath, layout.FromBaseImagePath(fullBaseImagePath))
						h.AssertNil(t, err)

						// add a random layer
						path, diffID, _ := h.RandomLayer(t, tmpDir)
						err = image.AddLayerWithDiffID(path, diffID)
						h.AssertNil(t, err)

						// save on disk in OCI
						err = image.Save("latest")
						h.AssertNil(t, err)

						// expected blobs: manifest, config, base image layer, new random layer
						h.AssertBlobsLen(t, imagePath, 4)

						// assert additional name
						index := h.ReadIndexManifest(t, imagePath)
						h.AssertEq(t, len(index.Manifests), 1)
						h.AssertEq(t, 1, len(index.Manifests[0].Annotations))
						h.AssertEqAnnotation(t, index.Manifests[0], layout.ImageRefNameKey, "latest")
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

						// save on disk in OCI
						err = image.Save("latest")
						h.AssertNil(t, err)

						// expected blobs: manifest, config, new random layer
						h.AssertBlobsLen(t, imagePath, 3)

						// assert additional name
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
				layerDiffID       string
				previousImagePath string
			)

			it.Before(func() {
				// value from testdata/layout/my-previous-image config.RootFS.DiffIDs
				layerDiffID = "sha256:ebc931a4ab83b0c934f2436c975cca387bc1bcebd1a5ced12824ff7592f317ea"
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
						err = image.ReuseLayer(layerDiffID)
						h.AssertNil(t, err)

						// save on disk in OCI
						err = image.Save("v1.0")
						h.AssertNil(t, err)

						// expected blobs: manifest, config, reuse random layer
						h.AssertBlobsLen(t, imagePath, 3)

						size, err := image.ManifestSize()
						h.AssertNil(t, err)
						h.AssertEq(t, size, int64(417))
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
						err = image.ReuseLayer(layerDiffID)
						h.AssertNil(t, err)

						// save on disk in OCI
						err = image.Save("v1.0")
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
