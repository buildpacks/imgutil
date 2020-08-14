package remote_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/remote"
	h "github.com/buildpacks/imgutil/testhelpers"
)

var dockerRegistry *h.DockerRegistry

func newTestImageName(providedPrefix ...string) string {
	prefix := "pack-image-test"
	if len(providedPrefix) > 0 {
		prefix = providedPrefix[0]
	}

	return dockerRegistry.RepoName(prefix + "-" + h.RandString(10))
}

func TestRemote(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())

	dockerConfigDir, err := ioutil.TempDir("", "test.docker.config.dir")
	h.AssertNil(t, err)
	defer os.RemoveAll(dockerConfigDir)

	dockerRegistry = h.NewDockerRegistryWithAuth(dockerConfigDir)
	dockerRegistry.Start(t)
	defer dockerRegistry.Stop(t)

	os.Setenv("DOCKER_CONFIG", dockerRegistry.DockerDirectory)
	defer os.Unsetenv("DOCKER_CONFIG")

	spec.Run(t, "Image", testImage, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testImage(t *testing.T, when spec.G, it spec.S) {
	var repoName string

	it.Before(func() {
		repoName = newTestImageName()
	})

	when("#NewRemote", func() {
		when("no base image is given", func() {
			it("returns an empty image", func() {
				_, err := remote.NewImage(newTestImageName(), authn.DefaultKeychain)
				h.AssertNil(t, err)
			})

			it("sets sensible defaults for all required fields", func() {
				// os, architecture, and rootfs are required per https://github.com/opencontainers/image-spec/blob/master/config.md
				img, err := remote.NewImage(newTestImageName(), authn.DefaultKeychain)
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

		when("#FromRemoteBaseImage", func() {
			when("base image exists", func() {
				it("sets the initial state from a linux/arm base image", func() {
					baseImageName := "arm64v8/busybox@sha256:50edf1d080946c6a76989d1c3b0e753b62f7d9b5f5e66e88bef23ebbd1e9709c"
					existingLayerSha := "sha256:5a0b973aa300cd2650869fd76d8546b361fcd6dfc77bd37b9d4f082cca9874e4"

					img, err := remote.NewImage(
						repoName,
						authn.DefaultKeychain,
						remote.FromBaseImage(baseImageName),
					)
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

				it("sets the initial state from a windows/amd64 base image", func() {
					baseImageName := "mcr.microsoft.com/windows/nanoserver@sha256:06281772b6a561411d4b338820d94ab1028fdeb076c85350bbc01e80c4bfa2b4"
					existingLayerSha := "sha256:26fd2d9d4c64a4f965bbc77939a454a31b607470f430b5d69fc21ded301fa55e"

					img, err := remote.NewImage(
						repoName,
						authn.DefaultKeychain,
						remote.FromBaseImage(baseImageName),
					)
					h.AssertNil(t, err)

					os, err := img.OS()
					h.AssertNil(t, err)
					h.AssertEq(t, os, "windows")

					osVersion, err := img.OSVersion()
					h.AssertNil(t, err)
					h.AssertEq(t, osVersion, "10.0.17763.1040")

					arch, err := img.Architecture()
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					readCloser, err := img.GetLayer(existingLayerSha)
					h.AssertNil(t, err)
					defer readCloser.Close()
				})

				when("base image is a multi-OS/Arch manifest list", func() {
					it("returns a base image matching the runtime GOOS/GOARCH", func() {
						manifestListName := "golang:1.13.8"
						existingLayerSha := "sha256:427da4a135b0869c1a274ba38e23d45bdbda93134c4ad99c8900cb0cfe9f0c9e"

						img, err := remote.NewImage(
							repoName,
							authn.DefaultKeychain,
							remote.FromBaseImage(manifestListName),
						)
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
			})

			when("base image does not exist", func() {
				it("don't error", func() {
					_, err := remote.NewImage(
						repoName,
						authn.DefaultKeychain,
						remote.FromBaseImage("some-bad-repo-name"),
					)

					h.AssertNil(t, err)
				})
			})
		})

		when("#WithPreviousImage", func() {
			when("previous image does not exist", func() {
				it("don't error", func() {
					_, err := remote.NewImage(
						repoName,
						authn.DefaultKeychain,
						remote.WithPreviousImage("some-bad-repo-name"),
					)

					h.AssertNil(t, err)
				})
			})
		})
	})

	when("#Label", func() {
		when("image exists", func() {
			it.Before(func() {
				baseImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("mykey", "myvalue"))
				h.AssertNil(t, baseImage.SetLabel("other", "data"))
				h.AssertNil(t, baseImage.Save())
			})

			it("returns the label value", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
				h.AssertNil(t, err)

				label, err := img.Label("mykey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "myvalue")
			})

			it("returns an empty string for a missing label", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
				h.AssertNil(t, err)

				label, err := img.Label("missing-label")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "")
			})
		})

		when("image is empty", func() {
			it("returns an empty label", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				label, err := img.Label("some-label")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "")
			})
		})
	})

	when("#Env", func() {
		when("image exists", func() {
			var baseImageName = newTestImageName()

			it.Before(func() {
				baseImage, err := remote.NewImage(baseImageName, authn.DefaultKeychain)
				h.AssertNil(t, err)
				h.AssertNil(t, baseImage.SetEnv("MY_VAR", "my_val"))
				h.AssertNil(t, baseImage.Save())
			})

			it("returns the label value", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(baseImageName))
				h.AssertNil(t, err)

				val, err := img.Env("MY_VAR")
				h.AssertNil(t, err)
				h.AssertEq(t, val, "my_val")
			})

			it("returns an empty string for a missing label", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(baseImageName))
				h.AssertNil(t, err)

				val, err := img.Env("MISSING_VAR")
				h.AssertNil(t, err)
				h.AssertEq(t, val, "")
			})
		})

		when("image is empty", func() {
			it("returns an empty string", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				val, err := img.Env("SOME_VAR")
				h.AssertNil(t, err)
				h.AssertEq(t, val, "")
			})
		})
	})

	when("#ListEnv", func() {
		when("image exists", func() {
			var baseImageName = newTestImageName()

			it.Before(func() {
				baseImage, err := remote.NewImage(baseImageName, authn.DefaultKeychain)
				h.AssertNil(t, err)
				h.AssertNil(t, baseImage.SetEnv("MY_VAR", "my_val"))
				h.AssertNil(t, baseImage.Save())
			})

			it("returns the environment", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(baseImageName))
				h.AssertNil(t, err)

				env, err := img.ListEnv()
				h.AssertNil(t, err)
				h.AssertEq(t, env, []string{"MY_VAR=my_val"})
			})
		})

		when("image is empty", func() {
			it("returns nil", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				env, err := img.ListEnv()
				h.AssertNil(t, err)
				h.AssertEq(t, env, []string(nil))
			})
		})
	})

	when("#Name", func() {
		it("always returns the original name", func() {
			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)
			h.AssertEq(t, img.Name(), repoName)
		})
	})

	when("#CreatedAt", func() {
		const reference = "busybox@sha256:f79f7a10302c402c052973e3fa42be0344ae6453245669783a9e16da3d56d5b4"
		it("returns the containers created at time", func() {
			img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(reference))
			h.AssertNil(t, err)

			expectedTime := time.Date(2019, 4, 2, 23, 32, 10, 727183061, time.UTC)

			createdTime, err := img.CreatedAt()

			h.AssertNil(t, err)
			h.AssertEq(t, createdTime, expectedTime)
		})
	})

	when("#Identifier", func() {
		it("returns a digest reference", func() {
			img, err := remote.NewImage(
				repoName+":some-tag",
				authn.DefaultKeychain,
				remote.FromBaseImage("busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"),
			)
			h.AssertNil(t, err)

			identifier, err := img.Identifier()
			h.AssertNil(t, err)
			h.AssertEq(t, identifier.String(), repoName+"@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5")
		})

		it("accurately parses the reference for an image with a sha", func() {
			img, err := remote.NewImage(
				repoName+"@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5",
				authn.DefaultKeychain,
				remote.FromBaseImage("busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"),
			)
			h.AssertNil(t, err)

			identifier, err := img.Identifier()
			h.AssertNil(t, err)
			h.AssertEq(t, identifier.String(), repoName+"@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5")
		})

		when("the image has been modified and saved", func() {
			it("returns the new digest reference", func() {
				img, err := remote.NewImage(
					repoName+":some-tag",
					authn.DefaultKeychain,
					remote.FromBaseImage("busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"),
				)
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("new", "label"))

				h.AssertNil(t, img.Save())

				id, err := img.Identifier()
				h.AssertNil(t, err)

				testImg, err := remote.NewImage(
					"test",
					authn.DefaultKeychain,
					remote.FromBaseImage(id.String()),
				)
				h.AssertNil(t, err)

				remoteLabel, err := testImg.Label("new")
				h.AssertNil(t, err)

				h.AssertEq(t, remoteLabel, "label")
			})
		})
	})

	when("#SetLabel", func() {
		when("image exists", func() {
			it("sets label on img object", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("mykey", "new-val"))
				label, err := img.Label("mykey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "new-val")
			})

			it("saves label", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("mykey", "new-val"))

				h.AssertNil(t, img.Save())

				testImg, err := remote.NewImage(
					"test",
					authn.DefaultKeychain,
					remote.FromBaseImage(repoName),
				)
				h.AssertNil(t, err)

				remoteLabel, err := testImg.Label("mykey")
				h.AssertNil(t, err)

				h.AssertEq(t, remoteLabel, "new-val")
			})
		})
	})

	when("#SetEnv", func() {
		it("sets the environment", func() {
			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetEnv("ENV_KEY", "ENV_VAL")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			configFile := h.FetchManifestImageConfigFile(t, repoName)
			h.AssertContains(t, configFile.Config.Env, "ENV_KEY=ENV_VAL")
		})
	})

	when("#SetWorkingDir", func() {
		it("sets the environment", func() {
			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetWorkingDir("/some/work/dir")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			configFile := h.FetchManifestImageConfigFile(t, repoName)
			h.AssertEq(t, configFile.Config.WorkingDir, "/some/work/dir")
		})
	})

	when("#SetEntrypoint", func() {
		it("sets the entrypoint", func() {
			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetEntrypoint("some", "entrypoint")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			configFile := h.FetchManifestImageConfigFile(t, repoName)
			h.AssertEq(t, configFile.Config.Entrypoint, []string{"some", "entrypoint"})
		})
	})

	when("#SetCmd", func() {
		it("sets the cmd", func() {
			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetCmd("some", "cmd")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			configFile := h.FetchManifestImageConfigFile(t, repoName)
			h.AssertEq(t, configFile.Config.Cmd, []string{"some", "cmd"})
		})
	})

	when("#Rebase", func() {
		when("image exists", func() {
			var oldBase, newBase, oldTopLayerDiffID string
			var oldBaseLayers, newBaseLayers, repoTopLayers []string
			it.Before(func() {
				// new base
				newBase = newTestImageName("pack-newbase-test")
				newBaseLayer1Path, err := h.CreateSingleFileLayerTar("/new-base.txt", "new-base", "linux")
				h.AssertNil(t, err)
				defer os.Remove(newBaseLayer1Path)

				newBaseLayer2Path, err := h.CreateSingleFileLayerTar("/otherfile.txt", "text-new-base", "linux")
				h.AssertNil(t, err)
				defer os.Remove(newBaseLayer2Path)

				newBaseImage, err := remote.NewImage(newBase, authn.DefaultKeychain)
				h.AssertNil(t, err)

				err = newBaseImage.AddLayer(newBaseLayer1Path)
				h.AssertNil(t, err)

				err = newBaseImage.AddLayer(newBaseLayer2Path)
				h.AssertNil(t, err)

				h.AssertNil(t, newBaseImage.Save())

				newBaseLayers = h.FetchManifestLayers(t, newBase)

				// old base image
				oldBase = newTestImageName("pack-oldbase-test")
				oldBaseLayer1Path, err := h.CreateSingleFileLayerTar("/old-base.txt", "old-base", "linux")
				h.AssertNil(t, err)
				defer os.Remove(oldBaseLayer1Path)

				oldBaseLayer2Path, err := h.CreateSingleFileLayerTar("/otherfile.txt", "text-old-base", "linux")
				h.AssertNil(t, err)
				defer os.Remove(oldBaseLayer2Path)

				oldBaseImage, err := remote.NewImage(oldBase, authn.DefaultKeychain)
				h.AssertNil(t, err)

				err = oldBaseImage.AddLayer(oldBaseLayer1Path)
				h.AssertNil(t, err)

				err = oldBaseImage.AddLayer(oldBaseLayer2Path)
				h.AssertNil(t, err)

				oldTopLayerDiffID = h.FileDiffID(t, oldBaseLayer2Path)

				h.AssertNil(t, oldBaseImage.Save())

				oldBaseLayers = h.FetchManifestLayers(t, oldBase)

				// original image
				origLayer1Path, err := h.CreateSingleFileLayerTar("/bmyimage.txt", "text-from-image-1", "linux")
				h.AssertNil(t, err)
				defer os.Remove(origLayer1Path)

				origLayer2Path, err := h.CreateSingleFileLayerTar("/myimage2.txt", "text-from-image-2", "linux")
				h.AssertNil(t, err)
				defer os.Remove(origLayer2Path)

				origImage, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(oldBase))
				h.AssertNil(t, err)

				err = origImage.AddLayer(origLayer1Path)
				h.AssertNil(t, err)

				err = origImage.AddLayer(origLayer2Path)
				h.AssertNil(t, err)

				h.AssertNil(t, origImage.Save())

				repoLayers := h.FetchManifestLayers(t, repoName)
				repoTopLayers = repoLayers[len(oldBaseLayers):]
			})

			it("switches the base", func() {
				// Before
				h.AssertEq(t,
					h.FetchManifestLayers(t, repoName),
					append(oldBaseLayers, repoTopLayers...),
				)

				// Run rebase
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
				h.AssertNil(t, err)

				newBaseImg, err := remote.NewImage(newBase, authn.DefaultKeychain, remote.FromBaseImage(newBase))
				h.AssertNil(t, err)

				err = img.Rebase(oldTopLayerDiffID, newBaseImg)
				h.AssertNil(t, err)
				h.AssertNil(t, img.Save())

				// After
				h.AssertEq(t,
					h.FetchManifestLayers(t, repoName),
					append(newBaseLayers, repoTopLayers...),
				)

				newBaseConfig := h.FetchManifestImageConfigFile(t, newBase)
				rebasedImgConfig := h.FetchManifestImageConfigFile(t, repoName)
				h.AssertEq(t, rebasedImgConfig.OS, newBaseConfig.OS)
				h.AssertEq(t, rebasedImgConfig.OSVersion, newBaseConfig.OSVersion)
				h.AssertEq(t, rebasedImgConfig.Architecture, newBaseConfig.Architecture)
			})
		})
	})

	when("#TopLayer", func() {
		when("image exists", func() {
			it("returns the digest for the top layer (useful for rebasing)", func() {
				baseLayerPath, err := h.CreateSingleFileLayerTar("/old-base.txt", "old-base", "linux")
				h.AssertNil(t, err)
				defer os.Remove(baseLayerPath)

				topLayerPath, err := h.CreateSingleFileLayerTar("/top-layer.txt", "top-layer", "linux")
				h.AssertNil(t, err)
				defer os.Remove(topLayerPath)

				expectedTopLayerDiffID := h.FileDiffID(t, topLayerPath)

				existingImage, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
				h.AssertNil(t, err)

				err = existingImage.AddLayer(baseLayerPath)
				h.AssertNil(t, err)

				err = existingImage.AddLayer(topLayerPath)
				h.AssertNil(t, err)

				h.AssertNil(t, existingImage.Save())

				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
				h.AssertNil(t, err)

				actualTopLayerDiffID, err := img.TopLayer()
				h.AssertNil(t, err)

				h.AssertEq(t, actualTopLayerDiffID, expectedTopLayerDiffID)
			})
		})

		when("the image has no layers", func() {
			it("returns an error", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				_, err = img.TopLayer()
				h.AssertError(t, err, "has no layers")
			})
		})
	})

	when("#AddLayer", func() {
		it("appends a layer", func() {
			existingImage, err := remote.NewImage(
				repoName,
				authn.DefaultKeychain,
			)
			h.AssertNil(t, err)

			oldLayerPath, err := h.CreateSingleFileLayerTar("/old-layer.txt", "old-layer", "linux")
			h.AssertNil(t, err)
			defer os.Remove(oldLayerPath)

			oldLayerDiffID := h.FileDiffID(t, oldLayerPath)

			h.AssertNil(t, existingImage.AddLayer(oldLayerPath))

			h.AssertNil(t, existingImage.Save())
			img, err := remote.NewImage(
				repoName,
				authn.DefaultKeychain,
				remote.FromBaseImage(repoName),
			)
			h.AssertNil(t, err)

			newLayerPath, err := h.CreateSingleFileLayerTar("/new-layer.txt", "new-layer", "linux")
			h.AssertNil(t, err)
			defer os.Remove(newLayerPath)

			newLayerDiffID := h.FileDiffID(t, newLayerPath)

			err = img.AddLayer(newLayerPath)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			manifestLayerDiffIDs := h.FetchManifestLayers(t, repoName)

			h.AssertEq(t, oldLayerDiffID, manifestLayerDiffIDs[len(manifestLayerDiffIDs)-2])
			h.AssertEq(t, newLayerDiffID, manifestLayerDiffIDs[len(manifestLayerDiffIDs)-1])
		})
	})

	when("#AddLayerWithDiffID", func() {
		it("appends a layer", func() {
			existingImage, err := remote.NewImage(
				repoName,
				authn.DefaultKeychain,
			)
			h.AssertNil(t, err)

			oldLayerPath, err := h.CreateSingleFileLayerTar("/old-layer.txt", "old-layer", "linux")
			h.AssertNil(t, err)
			defer os.Remove(oldLayerPath)
			oldLayerDiffID := h.FileDiffID(t, oldLayerPath)

			h.AssertNil(t, existingImage.AddLayer(oldLayerPath))

			h.AssertNil(t, existingImage.Save())

			img, err := remote.NewImage(
				repoName,
				authn.DefaultKeychain,
				remote.FromBaseImage(repoName),
			)
			h.AssertNil(t, err)

			newLayerPath, err := h.CreateSingleFileLayerTar("/new-layer.txt", "new-layer", "linux")
			h.AssertNil(t, err)
			defer os.Remove(newLayerPath)

			newLayerDiffID := h.FileDiffID(t, newLayerPath)

			err = img.AddLayerWithDiffID(newLayerPath, newLayerDiffID)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			manifestLayerDiffIDs := h.FetchManifestLayers(t, repoName)

			h.AssertEq(t, oldLayerDiffID, manifestLayerDiffIDs[len(manifestLayerDiffIDs)-2])
			h.AssertEq(t, newLayerDiffID, manifestLayerDiffIDs[len(manifestLayerDiffIDs)-1])
		})
	})

	when("#ReuseLayer", func() {
		when("previous image", func() {
			var (
				prevImageName string
				prevLayer1SHA string
				prevLayer2SHA string
			)

			it.Before(func() {
				prevImageName = newTestImageName()
				prevImage, err := remote.NewImage(
					prevImageName,
					authn.DefaultKeychain,
				)
				h.AssertNil(t, err)

				layer1Path, err := h.CreateSingleFileLayerTar("/layer-1.txt", "old-layer-1", "linux")
				h.AssertNil(t, err)
				defer os.Remove(layer1Path)

				prevLayer1SHA = h.FileDiffID(t, layer1Path)

				layer2Path, err := h.CreateSingleFileLayerTar("/layer-2.txt", "old-layer-2", "linux")
				h.AssertNil(t, err)
				defer os.Remove(layer2Path)

				prevLayer2SHA = h.FileDiffID(t, layer2Path)

				h.AssertNil(t, prevImage.AddLayer(layer1Path))
				h.AssertNil(t, prevImage.AddLayer(layer2Path))

				h.AssertNil(t, prevImage.Save())
			})

			it("reuses a layer", func() {
				img, err := remote.NewImage(
					repoName,
					authn.DefaultKeychain,
					remote.WithPreviousImage(prevImageName),
				)
				h.AssertNil(t, err)

				newBaseLayerPath, err := h.CreateSingleFileLayerTar("/new-base.txt", "base-content", "linux")
				h.AssertNil(t, err)
				defer os.Remove(newBaseLayerPath)

				h.AssertNil(t, img.AddLayer(newBaseLayerPath))

				err = img.ReuseLayer(prevLayer2SHA)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())

				manifestLayers := h.FetchManifestLayers(t, repoName)

				newLayer1SHA := manifestLayers[len(manifestLayers)-2]
				reusedLayer2SHA := manifestLayers[len(manifestLayers)-1]

				h.AssertNotEq(t, prevLayer1SHA, newLayer1SHA)
				h.AssertEq(t, prevLayer2SHA, reusedLayer2SHA)
			})

			it("returns error on nonexistent layer", func() {
				img, err := remote.NewImage(
					repoName,
					authn.DefaultKeychain,
					remote.WithPreviousImage(prevImageName),
				)
				h.AssertNil(t, err)

				img.Rename(repoName)

				err = img.ReuseLayer("some-bad-sha")

				h.AssertError(t, err, "previous image did not have layer with diff id 'some-bad-sha'")
			})
		})
	})

	when("#Save", func() {
		when("image exists", func() {
			it("can be pulled by digest", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				err = img.SetLabel("mykey", "newValue")
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())

				identifier, err := img.Identifier()
				h.AssertNil(t, err)

				testImg, err := remote.NewImage(
					"test",
					authn.DefaultKeychain,
					remote.FromBaseImage(identifier.String()),
				)
				h.AssertNil(t, err)

				remoteLabel, err := testImg.Label("mykey")
				h.AssertNil(t, err)

				h.AssertEq(t, remoteLabel, "newValue")
			})

			it("zeroes all times and client specific fields", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				tarPath, err := h.CreateSingleFileLayerTar("/new-layer.txt", "new-layer", "linux")
				h.AssertNil(t, err)
				defer os.Remove(tarPath)

				h.AssertNil(t, img.AddLayer(tarPath))

				h.AssertNil(t, img.Save())

				configFile := h.FetchManifestImageConfigFile(t, repoName)

				h.AssertEq(t, configFile.Created.Time, imgutil.NormalizedDateTime)
				h.AssertEq(t, configFile.Container, "")
				h.AssertEq(t, configFile.DockerVersion, "")

				h.AssertEq(t, len(configFile.History), len(configFile.RootFS.DiffIDs))
				for _, item := range configFile.History {
					h.AssertEq(t, item.Created.Unix(), imgutil.NormalizedDateTime.Unix())
				}
			})
		})

		when("additional names are provided", func() {
			var (
				repoName            = newTestImageName()
				additionalRepoNames = []string{
					repoName + ":" + h.RandString(5),
					newTestImageName(),
					newTestImageName(),
				}
				successfulRepoNames = append([]string{repoName}, additionalRepoNames...)
			)

			it("saves to multiple names", func() {
				image, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, image.Save(additionalRepoNames...))
				for _, n := range successfulRepoNames {
					testImg, err := remote.NewImage(n, authn.DefaultKeychain)
					h.AssertNil(t, err)
					h.AssertEq(t, testImg.Found(), true)
				}
			})

			when("a single image name fails", func() {
				it("returns results with errors for those that failed", func() {
					failingName := newTestImageName() + ":🧨"

					image, err := remote.NewImage(repoName, authn.DefaultKeychain)
					h.AssertNil(t, err)

					err = image.Save(append([]string{failingName}, additionalRepoNames...)...)
					h.AssertError(t, err, fmt.Sprintf("failed to write image to the following tags: [%s:", failingName))

					// check all but failing name
					saveErr, ok := err.(imgutil.SaveError)
					h.AssertEq(t, ok, true)
					h.AssertEq(t, len(saveErr.Errors), 1)
					h.AssertEq(t, saveErr.Errors[0].ImageName, failingName)
					h.AssertError(t, saveErr.Errors[0].Cause, "could not parse reference")

					for _, n := range successfulRepoNames {
						testImg, err := remote.NewImage(n, authn.DefaultKeychain)
						h.AssertNil(t, err)
						h.AssertEq(t, testImg.Found(), true)
					}
				})
			})
		})
	})

	when("#Found", func() {
		when("it exists", func() {
			it("returns true, nil", func() {
				origImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)
				h.AssertNil(t, origImage.Save())

				image, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertEq(t, image.Found(), true)
			})
		})

		when("it does not exist", func() {
			it("returns false, nil", func() {
				image, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertEq(t, image.Found(), false)
			})
		})
	})

	when("#Delete", func() {
		when("it exists", func() {
			var img imgutil.Image
			it("returns nil and is deleted", func() {
				origImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)
				h.AssertNil(t, origImage.SetLabel("some-label", "some-val"))
				h.AssertNil(t, origImage.Save())

				img, err = remote.NewImage(
					repoName,
					authn.DefaultKeychain,
					remote.FromBaseImage(repoName),
				)

				h.AssertNil(t, err)
				h.AssertEq(t, img.Found(), true)

				h.AssertNil(t, img.Delete())

				h.AssertEq(t, img.Found(), false)
			})
		})

		when("it does not exists", func() {
			it("returns an error", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertEq(t, img.Found(), false)
				h.AssertError(t, img.Delete(), "MANIFEST_UNKNOWN")
			})
		})
	})
}
