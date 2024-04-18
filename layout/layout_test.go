package layout_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/layout"
	imgutilRemote "github.com/buildpacks/imgutil/remote"
	h "github.com/buildpacks/imgutil/testhelpers"
)

// FIXME: relevant tests in this file should be moved into new_test.go and save_test.go to mirror the implementation
func TestLayout(t *testing.T) {
	dockerConfigDir, err := os.MkdirTemp("", "test.docker.config.dir")
	h.AssertNil(t, err)
	defer os.RemoveAll(dockerConfigDir)

	dockerRegistry = h.NewDockerRegistry(h.WithAuth(dockerConfigDir))
	dockerRegistry.Start(t)
	defer dockerRegistry.Stop(t)

	os.Setenv("DOCKER_CONFIG", dockerConfigDir)
	defer os.Unsetenv("DOCKER_CONFIG")

	spec.Run(t, "Image", testImage, spec.Parallel(), spec.Report(report.Terminal{}))
	spec.Run(t, "ImageIndex", testImageIndex, spec.Parallel(), spec.Report(report.Terminal{}))
}

var (
	dockerRegistry *h.DockerRegistry

	// global directory and paths
	testDataDir = filepath.Join("testdata", "layout")
)

func testImage(t *testing.T, when spec.G, it spec.S) {
	var (
		remoteBaseImage     v1.Image
		tmpDir              string
		imagePath           string
		fullBaseImagePath   string
		sparseBaseImagePath string
		err                 error
	)

	it.Before(func() {
		// creates a v1.Image from a remote repository
		remoteBaseImage = h.RemoteRunnableBaseImage(t)

		// creates the directory to save all the OCI images on disk
		tmpDir, err = os.MkdirTemp("", "layout-test-files")
		h.AssertNil(t, err)

		imagePath, err = os.MkdirTemp("", "layout-test-image")
		h.AssertNil(t, err)

		// global directory and paths
		testDataDir = filepath.Join("testdata", "layout")
		fullBaseImagePath = filepath.Join(testDataDir, "busybox")
		sparseBaseImagePath = filepath.Join(testDataDir, "busybox-sparse")
	})

	it.After(func() {
		// removes all images created
		os.RemoveAll(tmpDir)
		os.RemoveAll(imagePath)
	})

	when("#NewImage", func() {
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
				h.AssertNil(t, err) // Window images include a runnable base layer
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

		when("#FromBaseImageInstance", func() {
			when("no platform is specified", func() {
				when("base image is provided", func() {
					it.Before(func() {
						var opts []remote.Option
						remoteBaseImage = h.RemoteImage(t, "arm64v8/busybox@sha256:50edf1d080946c6a76989d1c3b0e753b62f7d9b5f5e66e88bef23ebbd1e9709c", opts)
					})

					it("sets the initial state from a linux/arm base image", func() {
						existingLayerSha := "sha256:5a0b973aa300cd2650869fd76d8546b361fcd6dfc77bd37b9d4f082cca9874e4"

						img, err := layout.NewImage(imagePath, layout.FromBaseImageInstance(remoteBaseImage), layout.WithMediaTypes(imgutil.OCITypes))
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
						img, err := layout.NewImage(imagePath, layout.FromBaseImageInstance(nil))
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

			when("existing config has extra fields", func() {
				it("returns an unmodified digest", func() {
					img, err := layout.NewImage(imagePath, layout.FromBaseImagePath(filepath.Join("testdata", "layout", "busybox-sparse")))
					h.AssertNil(t, err)
					digest, err := img.Digest()
					h.AssertNil(t, err)
					h.AssertEq(t, digest.String(), "sha256:f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab")
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

			when("using a (sparse) base image", func() {
				it("sets the requested media types", func() {
					img, err := layout.NewImage(
						imagePath,
						layout.FromBaseImagePath(sparseBaseImagePath),
						layout.WithMediaTypes(imgutil.OCITypes),
					)
					h.AssertNil(t, err)
					h.AssertOCIMediaTypes(t, img) // before saving
					// add a random layer
					path, diffID, _ := h.RandomLayer(t, tmpDir)
					err = img.AddLayerWithDiffID(path, diffID)
					h.AssertNil(t, err)
					h.AssertOCIMediaTypes(t, img) // after adding a layer
					h.AssertNil(t, img.Save())
					h.AssertOCIMediaTypes(t, img) // after saving
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
				previousImagePath = filepath.Join(testDataDir, "my-previous-image")
			})

			when("previous image exists", func() {
				when("previous image is not sparse", func() {
					it.Before(func() {
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
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
					layout.FromBaseImageInstance(img),
				)
				h.AssertNil(t, err)

				layoutLabel, err := testImg.Label("mykey")
				h.AssertNil(t, err)

				h.AssertEq(t, layoutLabel, "new-val")
			})
		})
	})

	when("#RemoveLabel", func() {
		when("image exists", func() {
			var baseImageNamePath string

			it.Before(func() {
				tmpBaseImageDir, err := os.MkdirTemp(tmpDir, "my-base-image")
				h.AssertNil(t, err)

				baseImageNamePath = filepath.Join(tmpBaseImageDir, "my-base-image")

				baseImage, err := layout.NewImage(baseImageNamePath, layout.FromBaseImageInstance(remoteBaseImage))
				h.AssertNil(t, err)
				h.AssertNil(t, baseImage.SetLabel("custom.label", "new-val"))
				h.AssertNil(t, baseImage.Save())
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
					layout.FromBaseImageInstance(img),
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
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
		when("#FromBaseImageInstance with full image", func() {
			when("additional names are provided", func() {
				it("creates an image and save it to both path provided", func() {
					image, err := layout.NewImage(imagePath, layout.FromBaseImageInstance(remoteBaseImage))
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
					image, err := layout.NewImage(imagePath, layout.FromBaseImageInstance(remoteBaseImage))
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
				previousImagePath = filepath.Join(testDataDir, "my-previous-image")
			})

			when("previous image is not sparse", func() {
				it.Before(func() {
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		when("image doesn't exist on disk", func() {
			it.Before(func() {
				localPath := filepath.Join(tmpDir, "non-exist-image")
				image, err = layout.NewImage(localPath)
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
				localPath := filepath.Join(testDataDir, "my-previous-image")
				image, err = layout.NewImage(localPath)
				h.AssertNil(t, err)
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)
		})

		when("image doesn't exist on disk", func() {
			it.Before(func() {
				localPath := filepath.Join(tmpDir, "non-exist-image")
				image, err = layout.NewImage(localPath)
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
				localPath := filepath.Join(testDataDir, "my-previous-image")
				image, err = layout.NewImage(localPath)
				h.AssertNil(t, err)
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
			image, err = layout.NewImage(imagePath)
			h.AssertNil(t, err)

			platform = imgutil.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "5678",
			}
		})

		it("Platform values are saved on disk in OCI layout format", func() {
			var (
				os         = "linux"
				arch       = "amd64"
				variant    = "some-variant"
				osVersion  = "1234"
				osFeatures = []string{"some-osFeatures"}
				annos      = map[string]string{"some-key": "some-value"}
			)
			h.AssertNil(t, image.SetOS(os))
			h.AssertNil(t, image.SetArchitecture(arch))
			h.AssertNil(t, image.SetVariant(variant))
			h.AssertNil(t, image.SetOSVersion(osVersion))
			h.AssertNil(t, image.SetOSFeatures(osFeatures))
			h.AssertNil(t, image.SetAnnotations(annos))

			h.AssertNil(t, image.Save())

			manifestFile, configFile := h.ReadManifestAndConfigFile(t, imagePath)
			h.AssertEq(t, configFile.OS, os)
			h.AssertEq(t, configFile.Architecture, arch)
			h.AssertEq(t, configFile.Variant, variant)
			h.AssertEq(t, configFile.OSVersion, osVersion)
			h.AssertEq(t, configFile.OSFeatures, osFeatures)
			h.AssertEq(t, manifestFile.Annotations, annos)
		})

		it("Default Platform values are saved on disk in OCI layout format", func() {
			image, err = layout.NewImage(imagePath, layout.WithDefaultPlatform(platform))
			h.AssertNil(t, err)

			image.Save()

			_, configFile := h.ReadManifestAndConfigFile(t, imagePath)
			h.AssertEq(t, configFile.OS, platform.OS)
			h.AssertEq(t, configFile.Architecture, platform.Architecture)
			h.AssertEq(t, configFile.OSVersion, platform.OSVersion)
		})
	})

	when("#GetLayer", func() {
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

func testImageIndex(t *testing.T, when spec.G, it spec.S) {
	var (
		idx           imgutil.ImageIndex
		tmpDir        string
		localPath     string
		baseIndexPath string
		err           error
	)

	it.Before(func() {
		// creates the directory to save all the OCI images on disk
		tmpDir, err = os.MkdirTemp("", "layout-image-indexes")
		h.AssertNil(t, err)

		// image index directory on disk
		baseIndexPath = filepath.Join(testDataDir, "busybox-multi-platform")
		// global directory and paths
		testDataDir = filepath.Join("testdata", "layout")
	})

	it.After(func() {
		err := os.RemoveAll(tmpDir)
		h.AssertNil(t, err)
	})

	when("Getters", func() {
		var (
			attribute   string
			attributes  []string
			annotations map[string]string
			digest      name.Digest
		)
		when("index exists on disk", func() {
			when("#FromBaseImageIndex", func() {
				it.Before(func() {
					idx, err = layout.NewIndex("busybox-multi-platform", tmpDir, imgutil.FromBaseImageIndex(baseIndexPath))
					h.AssertNil(t, err)
					localPath = filepath.Join(tmpDir, "busybox-multi-platform")
				})

				// See spec: https://github.com/opencontainers/image-spec/blob/main/image-index.md#image-index-property-descriptions
				when("linux/amd64", func() {
					it.Before(func() {
						digest, err = name.NewDigest("busybox-multi-platform@sha256:4be429a5fbb2e71ae7958bfa558bc637cf3a61baf40a708cb8fff532b39e52d0")
						h.AssertNil(t, err)
					})

					it("existing platform attributes are readable", func() {
						// #Architecture
						attribute, err = idx.Architecture(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "amd64")

						// #OS
						attribute, err = idx.OS(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "linux")

						// #Variant
						attribute, err = idx.Variant(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "v1")

						// #OSVersion
						attribute, err = idx.OSVersion(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "4.5.6")

						// #OSFeatures
						attributes, err = idx.OSFeatures(digest)
						h.AssertNil(t, err)
						h.AssertContains(t, attributes, "os-feature-1", "os-feature-2")
					})

					it("existing annotations are readable", func() {
						annotations, err = idx.Annotations(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, annotations["com.docker.official-images.bashbrew.arch"], "amd64")
						h.AssertEq(t, annotations["org.opencontainers.image.url"], "https://hub.docker.com/_/busybox")
						h.AssertEq(t, annotations["org.opencontainers.image.revision"], "d0b7d566eb4f1fa9933984e6fc04ab11f08f4592")
					})
				})

				when("linux/arm64", func() {
					it.Before(func() {
						digest, err = name.NewDigest("busybox-multi-platform@sha256:8a4415fb43600953cbdac6ec03c2d96d900bb21f8d78964837dad7f73b9afcdc")
						h.AssertNil(t, err)
					})

					it("existing platform attributes are readable", func() {
						// #Architecture
						attribute, err = idx.Architecture(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "arm")

						// #OS
						attribute, err = idx.OS(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "linux")

						// #Variant
						attribute, err = idx.Variant(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "v7")

						// #OSVersion
						attribute, err = idx.OSVersion(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "1.2.3")

						// #OSFeatures
						attributes, err = idx.OSFeatures(digest)
						h.AssertNil(t, err)
						h.AssertContains(t, attributes, "os-feature-3", "os-feature-4")
					})

					it("existing annotations are readable", func() {
						annotations, err = idx.Annotations(digest)
						h.AssertNil(t, err)
						h.AssertEq(t, annotations["com.docker.official-images.bashbrew.arch"], "arm32v7")
						h.AssertEq(t, annotations["org.opencontainers.image.url"], "https://hub.docker.com/_/busybox")
						h.AssertEq(t, annotations["org.opencontainers.image.revision"], "185a3f7f21c307b15ef99b7088b228f004ff5f11")
					})
				})
			})
		})
	})

	when("#Save", func() {
		when("index exists on disk", func() {
			when("#FromBaseImageIndex", func() {
				it.Before(func() {
					idx, err = layout.NewIndex("busybox-multi-platform", tmpDir, imgutil.FromBaseImageIndex(baseIndexPath))
					h.AssertNil(t, err)

					localPath = filepath.Join(tmpDir, "busybox-multi-platform")
				})

				it("manifests from base image index are saved on disk", func() {
					err = idx.SaveDir()
					h.AssertNil(t, err)

					// assert linux/amd64 and linux/arm64 manifests were saved
					index := h.ReadIndexManifest(t, localPath)
					h.AssertEq(t, len(index.Manifests), 2)
					h.AssertEq(t, index.Manifests[0].Digest.String(), "sha256:4be429a5fbb2e71ae7958bfa558bc637cf3a61baf40a708cb8fff532b39e52d0")
					h.AssertEq(t, index.Manifests[1].Digest.String(), "sha256:8a4415fb43600953cbdac6ec03c2d96d900bb21f8d78964837dad7f73b9afcdc")
				})
			})

			when("#FromBaseImageIndexInstance", func() {
				it.Before(func() {
					localIndex := h.ReadImageIndex(t, baseIndexPath)

					idx, err = layout.NewIndex("busybox-multi-platform", tmpDir, imgutil.FromBaseImageIndexInstance(localIndex))
					h.AssertNil(t, err)

					localPath = filepath.Join(tmpDir, "busybox-multi-platform")
				})

				it("manifests from base image index instance are saved on disk", func() {
					err = idx.SaveDir()
					h.AssertNil(t, err)

					// assert linux/amd64 and linux/arm64 manifests were saved
					index := h.ReadIndexManifest(t, localPath)
					h.AssertEq(t, len(index.Manifests), 2)
					h.AssertEq(t, index.Manifests[0].Digest.String(), "sha256:4be429a5fbb2e71ae7958bfa558bc637cf3a61baf40a708cb8fff532b39e52d0")
					h.AssertEq(t, index.Manifests[1].Digest.String(), "sha256:8a4415fb43600953cbdac6ec03c2d96d900bb21f8d78964837dad7f73b9afcdc")
				})
			})
		})
	})

	when("#Add", func() {
		var (
			imagePath         string
			fullBaseImagePath string
		)

		it.Before(func() {
			imagePath, err = os.MkdirTemp(tmpDir, "layout-test-image-index")
			h.AssertNil(t, err)

			fullBaseImagePath = filepath.Join(testDataDir, "busybox")
		})

		when("index is created from scratch", func() {
			it.Before(func() {
				repoName := newRepoName()
				idx = setUpImageIndex(t, repoName, tmpDir)
				localPath = filepath.Join(tmpDir, repoName)
			})

			when("manifest in OCI layout format is added", func() {
				var editableImage v1.Image
				it.Before(func() {
					editableImage, err = layout.NewImage(imagePath, layout.FromBaseImagePath(fullBaseImagePath))
					h.AssertNil(t, err)
				})

				it("adds the manifest to the index", func() {
					idx.AddManifest(editableImage)
					// manifest was added
					index := h.ReadIndexManifest(t, localPath)
					h.AssertEq(t, len(index.Manifests), 1)
				})
			})
		})

		when("index exists on disk", func() {
			when("#FromBaseImageIndex", func() {
				it.Before(func() {
					idx = setUpImageIndex(t, "busybox-multi-platform", tmpDir, imgutil.FromBaseImageIndex(baseIndexPath))
					localPath = filepath.Join(tmpDir, "busybox-multi-platform")
				})

				when("manifest in OCI layout format is added", func() {
					var editableImage v1.Image
					it.Before(func() {
						editableImage, err = layout.NewImage(imagePath, layout.FromBaseImagePath(fullBaseImagePath))
						h.AssertNil(t, err)
					})

					it("adds the manifest to the index", func() {
						idx.AddManifest(editableImage)
						index := h.ReadIndexManifest(t, localPath)
						// manifest was added
						// initially it has 2 manifest + 1 new
						h.AssertEq(t, len(index.Manifests), 3)
					})
				})
			})
		})
	})

	when("#Push", func() {
		when("index is created from scratch", func() {
			it.Before(func() {
				repoName := newTestImageIndexName("push-index-test")
				idx = setUpImageIndex(t, repoName, tmpDir, imgutil.WithKeychain(authn.DefaultKeychain))

				// TODO Note in the Push operation
				// Note: It will only push IndexManifest, assuming all the images it refers exists in registry
				// We need to push each individual image first]

				img1RepoName := fmt.Sprintf("%s:%s", repoName, "busybox-amd64")
				img1, err := imgutilRemote.NewImage(img1RepoName, authn.DefaultKeychain, imgutilRemote.FromBaseImage("busybox@sha256:4be429a5fbb2e71ae7958bfa558bc637cf3a61baf40a708cb8fff532b39e52d0"))
				h.AssertNil(t, err)
				err = img1.Save()
				h.AssertNil(t, err)

				idx.AddManifest(img1)

				img2RepoName := fmt.Sprintf("%s:%s", repoName, "busybox-arm64")
				img2, err := imgutilRemote.NewImage(img2RepoName, authn.DefaultKeychain, imgutilRemote.FromBaseImage("busybox@sha256:8a4415fb43600953cbdac6ec03c2d96d900bb21f8d78964837dad7f73b9afcdc"))
				h.AssertNil(t, err)
				err = img2.Save()
				h.AssertNil(t, err)

				idx.AddManifest(img2)
			})

			it("image index is pushed", func() {
				err = idx.Push()
				h.AssertNil(t, err)
			})
		})
	})

	when("#Delete", func() {
		when("index exists on disk", func() {
			when("#FromBaseImageIndex", func() {
				it.Before(func() {
					idx = setUpImageIndex(t, "busybox-multi-platform", tmpDir, imgutil.FromBaseImageIndex(baseIndexPath))
					localPath = filepath.Join(tmpDir, "busybox-multi-platform")
				})

				it("deletes the imange index from disk", func() {
					// Verify the index exists
					h.ReadIndexManifest(t, localPath)

					err = idx.DeleteDir()
					h.AssertNil(t, err)

					_, err = os.Stat(localPath)
					h.AssertNotNil(t, err)
					h.AssertEq(t, true, os.IsNotExist(err))
				})
			})
		})
	})

	when("#Remove", func() {
		var digest name.Digest
		when("index exists on disk", func() {
			when("#FromBaseImageIndex", func() {
				it.Before(func() {
					idx = setUpImageIndex(t, "busybox-multi-platform", tmpDir, imgutil.FromBaseImageIndex(baseIndexPath), imgutil.WithKeychain(authn.DefaultKeychain))
					localPath = filepath.Join(tmpDir, "busybox-multi-platform")
					digest, err = name.NewDigest("busybox@sha256:4be429a5fbb2e71ae7958bfa558bc637cf3a61baf40a708cb8fff532b39e52d0")
					h.AssertNil(t, err)
				})

				it("given manifest is removed", func() {
					err = idx.RemoveManifest(digest)
					h.AssertNil(t, err)

					// After removing any operation to get something about the digest must fail
					_, err = idx.OS(digest)
					h.AssertNotNil(t, err)
					h.AssertError(t, err, "no image or image index found for digest")

					// After saving, the index on disk must reflect the change
					err = idx.SaveDir()
					h.AssertNil(t, err)

					index := h.ReadIndexManifest(t, localPath)
					h.AssertEq(t, len(index.Manifests), 1)
					h.AssertEq(t, index.Manifests[0].Digest.String(), "sha256:8a4415fb43600953cbdac6ec03c2d96d900bb21f8d78964837dad7f73b9afcdc")
				})
			})
		})
	})

	when("#Inspect", func() {
		var indexString string
		when("index exists on disk", func() {
			when("#FromBaseImageIndex", func() {
				it.Before(func() {
					idx = setUpImageIndex(t, "busybox-multi-platform", tmpDir, imgutil.FromBaseImageIndex(baseIndexPath))
					localPath = filepath.Join(tmpDir, "busybox-multi-platform")
				})

				it("returns an image index string representation", func() {
					indexString, err = idx.Inspect()
					h.AssertNil(t, err)

					idxFromString := parseImageIndex(t, indexString)
					h.AssertEq(t, len(idxFromString.Manifests), 2)
				})
			})
		})
	})
}

func setUpImageIndex(t *testing.T, repoName string, tmpDir string, ops ...imgutil.IndexOption) imgutil.ImageIndex {
	idx, err := layout.NewIndex(repoName, tmpDir, ops...)
	h.AssertNil(t, err)

	// TODO before adding something to the index, apparently we need initialize on disk
	err = idx.SaveDir()
	h.AssertNil(t, err)
	return idx
}

func newRepoName() string {
	return "test-layout-index-" + h.RandString(10)
}

func newTestImageIndexName(name string) string {
	return dockerRegistry.RepoName(name + "-" + h.RandString(10))
}

func parseImageIndex(t *testing.T, index string) *v1.IndexManifest {
	r := strings.NewReader(index)
	idx, err := v1.ParseIndexManifest(r)
	h.AssertNil(t, err)
	return idx
}
