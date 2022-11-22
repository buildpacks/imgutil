package layout_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/imgutil"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil/layout"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestImage(t *testing.T) {
	spec.Run(t, "LayoutImage", testImage, spec.Report(report.Terminal{}))
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
		testImage = h.CreateRemoteImage(t)

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

	when("#Save", func() {
		it.After(func() {
			os.RemoveAll(imagePath)
		})

		when("#FromBaseImage", func() {
			it.Before(func() {
				imagePath = filepath.Join(tmpDir, "save-from-base-image")
			})

			when("additional names are provided", func() {
				it("creates an image with all the layers from the underlying image and org.opencontainers.image.ref.name annotation", func() {
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
					h.AssertEq(t, "my-additional-tag", index.Manifests[0].Annotations["org.opencontainers.image.ref.name"])
				})

				it("failed on saved when more than one name is provided", func() {
					image, err := layout.NewImage(imagePath, layout.FromBaseImage(testImage))
					h.AssertNil(t, err)

					// save on disk in OCI
					err = image.Save("name1", "name2")
					h.AssertError(t, err, "multiple additional names [name1 name2] are not allow when OCI layout is used")
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
						err = image.Save()
						h.AssertNil(t, err)

						// expected blobs: manifest, config, base image layer, new random layer
						h.AssertBlobsLen(t, imagePath, 4)
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
						err = image.Save("v1.0")
						h.AssertNil(t, err)

						// expected blobs: manifest, config, new random layer
						h.AssertBlobsLen(t, imagePath, 3)

						// assert additional name
						index := h.ReadIndexManifest(t, imagePath)
						h.AssertEq(t, len(index.Manifests), 1)
						h.AssertEq(t, "v1.0", index.Manifests[0].Annotations["org.opencontainers.image.ref.name"])
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

	when("#Feature", func() {
		var image *layout.Image

		it.After(func() {
			os.RemoveAll(imagePath)
		})

		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "feature-image")
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		when("#Label(s)", func() {
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

		when("#Env(s)", func() {
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

		when("#WorkingDir", func() {
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

		when("#Found", func() {
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

		when("#CMD", func() {
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

		when("#Delete", func() {
			it.Before(func() {
				// Write the image on disk
				err = image.Save()
				h.AssertNil(t, err)

				// Image must be found
				h.AssertTrue(t, func() bool {
					return image.Found()
				})
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

			it.Before(func() {
				platform = imgutil.Platform{
					Architecture: "amd64",
					OS:           "linux",
					OSVersion:    "5678",
				}
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
	})
}
