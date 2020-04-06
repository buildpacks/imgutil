package remote_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/remote"
	h "github.com/buildpacks/imgutil/testhelpers"
)

var registryPort string

func newTestImageName() string {
	return "localhost:" + registryPort + "/pack-image-test-" + h.RandString(10)
}

func TestRemoteImage(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())

	dockerRegistry := h.NewDockerRegistry()
	dockerRegistry.Start(t)
	defer dockerRegistry.Stop(t)

	registryPort = dockerRegistry.Port

	spec.Run(t, "RemoteImage", testRemoteImage, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testRemoteImage(t *testing.T, when spec.G, it spec.S) {
	var repoName string
	var dockerClient client.CommonAPIClient

	it.Before(func() {
		var err error
		dockerClient = h.DockerCli(t)
		h.AssertNil(t, err)
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
				h.AssertNil(t, h.PullImage(dockerClient, img.Name()))
				defer h.DockerRmi(dockerClient, img.Name())
				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), img.Name())
				h.AssertNil(t, err)
				h.AssertEq(t, inspect.Os, "linux")
				h.AssertEq(t, inspect.Architecture, "amd64")
				h.AssertEq(t, inspect.RootFS.Type, "layers")
			})
		})

		when("#FromRemoteBaseImage", func() {
			when("base image exists", func() {
				var (
					baseName         = "busybox"
					err              error
					existingLayerSha string
				)

				it.Before(func() {
					err = h.PullImage(dockerClient, baseName)
					h.AssertNil(t, err)

					inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), baseName)
					h.AssertNil(t, err)

					existingLayerSha = inspect.RootFS.Layers[0]
				})

				it("sets the initial state to match the base image", func() {
					img, err := remote.NewImage(
						repoName,
						authn.DefaultKeychain,
						remote.FromBaseImage(baseName),
					)
					h.AssertNil(t, err)

					readCloser, err := img.GetLayer(existingLayerSha)
					h.AssertNil(t, err)
					defer readCloser.Close()
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
				var err error

				baseImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
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
				var err error

				baseImage, err := remote.NewImage(baseImageName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", baseImageName))
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
			var err error

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
			var err error

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
				var err error

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

				label := remoteLabel(t, dockerClient, id.String(), "new")
				h.AssertEq(t, "label", label)
			})
		})
	})

	when("#SetLabel", func() {
		when("image exists", func() {
			it("sets label on img object", func() {
				var err error

				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("mykey", "new-val"))
				label, err := img.Label("mykey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "new-val")
			})

			it("saves label", func() {
				var err error

				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("mykey", "new-val"))

				h.AssertNil(t, img.Save())

				// After Pull
				label := remoteLabel(t, dockerClient, repoName, "mykey")
				h.AssertEq(t, "new-val", label)
			})
		})
	})

	when("#SetEnv", func() {
		it("sets the environment", func() {
			var err error

			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetEnv("ENV_KEY", "ENV_VAL")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			h.AssertNil(t, h.PullImage(dockerClient, repoName))
			defer h.DockerRmi(dockerClient, repoName)

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertContains(t, inspect.Config.Env, "ENV_KEY=ENV_VAL")
		})
	})

	when("#SetWorkingDir", func() {
		it("sets the environment", func() {
			var err error

			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetWorkingDir("/some/work/dir")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			h.AssertNil(t, h.PullImage(dockerClient, repoName))
			defer h.DockerRmi(dockerClient, repoName)

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertEq(t, inspect.Config.WorkingDir, "/some/work/dir")
		})
	})

	when("#SetEntrypoint", func() {
		it("sets the entrypoint", func() {
			var err error

			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetEntrypoint("some", "entrypoint")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			h.AssertNil(t, h.PullImage(dockerClient, repoName))
			defer h.DockerRmi(dockerClient, repoName)

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertEq(t, []string(inspect.Config.Entrypoint), []string{"some", "entrypoint"})
		})
	})

	when("#SetCmd", func() {
		it("sets the cmd", func() {
			var err error

			img, err := remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)

			err = img.SetCmd("some", "cmd")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			h.AssertNil(t, h.PullImage(dockerClient, repoName))
			defer h.DockerRmi(dockerClient, repoName)

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertEq(t, []string(inspect.Config.Cmd), []string{"some", "cmd"})
		})
	})

	when("#Rebase", func() {
		when("image exists", func() {
			var oldBase, newBase, oldTopLayerDiffID string
			var oldBaseLayers, newBaseLayers, repoTopLayers []string
			it.Before(func() {
				// new base
				newBase = "localhost:" + registryPort + "/pack-newbase-test-" + h.RandString(10)
				newBaseLayer1Path, err := h.CreateSingleFileTar("/base.txt", "new-base")
				h.AssertNil(t, err)
				defer os.Remove(newBaseLayer1Path)

				newBaseLayer2Path, err := h.CreateSingleFileTar("/otherfile.txt", "text-new-base")
				h.AssertNil(t, err)
				defer os.Remove(newBaseLayer2Path)

				newBaseImage, err := remote.NewImage(newBase, authn.DefaultKeychain, remote.FromBaseImage("busybox"))
				h.AssertNil(t, err)

				err = newBaseImage.AddLayer(newBaseLayer1Path)
				h.AssertNil(t, err)

				err = newBaseImage.AddLayer(newBaseLayer2Path)
				h.AssertNil(t, err)

				h.AssertNil(t, newBaseImage.Save())

				newBaseLayers = manifestLayers(t, newBase)

				// old base image
				oldBase = "localhost:" + registryPort + "/pack-oldbase-test-" + h.RandString(10)
				oldBaseLayer1Path, err := h.CreateSingleFileTar("/base.txt", "old-base")
				h.AssertNil(t, err)
				defer os.Remove(oldBaseLayer1Path)

				oldBaseLayer2Path, err := h.CreateSingleFileTar("/otherfile.txt", "text-old-base")
				h.AssertNil(t, err)
				defer os.Remove(oldBaseLayer2Path)

				oldBaseImage, err := remote.NewImage(oldBase, authn.DefaultKeychain, remote.FromBaseImage("busybox"))
				h.AssertNil(t, err)

				err = oldBaseImage.AddLayer(oldBaseLayer1Path)
				h.AssertNil(t, err)

				err = oldBaseImage.AddLayer(oldBaseLayer2Path)
				h.AssertNil(t, err)

				oldTopLayerDiffID, err = h.FileDiffID(oldBaseLayer2Path)
				h.AssertNil(t, err)

				h.AssertNil(t, oldBaseImage.Save())

				oldBaseLayers = manifestLayers(t, oldBase)

				// original image
				origLayer1Path, err := h.CreateSingleFileTar("/bmyimage.txt", "text-from-image-1")
				h.AssertNil(t, err)
				defer os.Remove(origLayer1Path)

				origLayer2Path, err := h.CreateSingleFileTar("/myimage2.txt", "text-from-image-2")
				h.AssertNil(t, err)
				defer os.Remove(origLayer2Path)

				origImage, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(oldBase))
				h.AssertNil(t, err)

				err = origImage.AddLayer(origLayer1Path)
				h.AssertNil(t, err)

				err = origImage.AddLayer(origLayer2Path)
				h.AssertNil(t, err)

				h.AssertNil(t, origImage.Save())

				repoTopLayers = manifestLayers(t, repoName)[len(oldBaseLayers):]
			})

			it("switches the base", func() {
				var err error

				// Before
				h.AssertEq(t,
					manifestLayers(t, repoName),
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
					manifestLayers(t, repoName),
					append(newBaseLayers, repoTopLayers...),
				)
			})
		})
	})

	when("#TopLayer", func() {
		when("image exists", func() {
			it("returns the digest for the top layer (useful for rebasing)", func() {
				baseLayerPath, err := h.CreateSingleFileTar("/old-base.txt", "old-base")
				h.AssertNil(t, err)
				defer os.Remove(baseLayerPath)

				topLayerPath, err := h.CreateSingleFileTar("/top-layer.txt", "top-layer")
				h.AssertNil(t, err)
				defer os.Remove(topLayerPath)

				expectedTopLayerDiffID, err := h.FileDiffID(topLayerPath)
				h.AssertNil(t, err)

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
		it.Before(func() {
			var err error

			existingImage, err := remote.NewImage(
				repoName,
				authn.DefaultKeychain,
				remote.FromBaseImage("busybox"),
			)
			h.AssertNil(t, err)

			h.AssertNil(t, existingImage.SetLabel("repo_name_for_randomisation", repoName))

			oldLayerPath, err := h.CreateSingleFileTar("/old-layer.txt", "old-layer")
			h.AssertNil(t, err)
			defer os.Remove(oldLayerPath)

			h.AssertNil(t, existingImage.AddLayer(oldLayerPath))

			h.AssertNil(t, existingImage.Save())
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
		})

		it("appends a layer", func() {
			var err error

			newLayerPath, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer")
			h.AssertNil(t, err)
			defer os.Remove(newLayerPath)

			img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
			h.AssertNil(t, err)

			err = img.AddLayer(newLayerPath)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			// After Pull
			h.AssertNil(t, h.PullImage(dockerClient, repoName))

			output, err := h.CopySingleFileFromImage(dockerClient, repoName, "old-layer.txt")
			h.AssertNil(t, err)
			h.AssertEq(t, output, "old-layer")

			output, err = h.CopySingleFileFromImage(dockerClient, repoName, "new-layer.txt")
			h.AssertNil(t, err)
			h.AssertEq(t, output, "new-layer")
		})
	})

	when("#AddLayerWithDiffID", func() {
		it.Before(func() {
			var err error

			existingImage, err := remote.NewImage(
				repoName,
				authn.DefaultKeychain,
				remote.FromBaseImage("busybox"),
			)
			h.AssertNil(t, err)

			h.AssertNil(t, existingImage.SetLabel("repo_name_for_randomisation", repoName))

			oldLayerPath, err := h.CreateSingleFileTar("/old-layer.txt", "old-layer")
			h.AssertNil(t, err)
			defer os.Remove(oldLayerPath)

			h.AssertNil(t, existingImage.AddLayer(oldLayerPath))

			h.AssertNil(t, existingImage.Save())
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
		})

		it("appends a layer", func() {
			var err error

			img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
			h.AssertNil(t, err)

			newLayerPath, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer")
			h.AssertNil(t, err)
			defer os.Remove(newLayerPath)

			diffID, err := h.FileDiffID(newLayerPath)
			h.AssertNil(t, err)

			err = img.AddLayerWithDiffID(newLayerPath, diffID)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			// After Pull
			h.AssertNil(t, h.PullImage(dockerClient, repoName))

			output, err := h.CopySingleFileFromImage(dockerClient, repoName, "old-layer.txt")
			h.AssertNil(t, err)
			h.AssertEq(t, output, "old-layer")

			output, err = h.CopySingleFileFromImage(dockerClient, repoName, "new-layer.txt")
			h.AssertNil(t, err)
			h.AssertEq(t, output, "new-layer")
		})
	})

	when("#ReuseLayer", func() {
		when("previous image", func() {
			var (
				layer2SHA     string
				prevImageName string
			)

			it.Before(func() {
				var err error

				prevImageName = "localhost:" + registryPort + "/pack-image-test-" + h.RandString(10)
				prevImage, err := remote.NewImage(
					prevImageName,
					authn.DefaultKeychain,
				)
				h.AssertNil(t, err)

				h.AssertNil(t, prevImage.SetLabel("repo_name_for_randomisation", repoName))

				layer1Path, err := h.CreateSingleFileTar("/layer-1.txt", "old-layer-1")
				h.AssertNil(t, err)
				defer os.Remove(layer1Path)

				layer2Path, err := h.CreateSingleFileTar("/layer-2.txt", "old-layer-2")
				h.AssertNil(t, err)
				defer os.Remove(layer2Path)

				h.AssertNil(t, prevImage.AddLayer(layer1Path))
				h.AssertNil(t, prevImage.AddLayer(layer2Path))

				h.AssertNil(t, prevImage.Save())

				layer2SHA, err = h.FileDiffID(layer2Path)
				h.AssertNil(t, err)
			})

			it("reuses a layer", func() {
				var err error

				img, err := remote.NewImage(
					repoName,
					authn.DefaultKeychain,
					remote.WithPreviousImage(prevImageName),
					remote.FromBaseImage("busybox"),
				)
				h.AssertNil(t, err)

				err = img.ReuseLayer(layer2SHA)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())

				h.AssertNil(t, h.PullImage(dockerClient, repoName))
				defer h.DockerRmi(dockerClient, repoName)
				output, err := h.CopySingleFileFromImage(dockerClient, repoName, "layer-2.txt")
				h.AssertNil(t, err)
				h.AssertEq(t, output, "old-layer-2")

				// Confirm layer-1.txt does not exist
				_, err = h.CopySingleFileFromImage(dockerClient, repoName, "layer-1.txt")
				h.AssertMatch(t, err.Error(), regexp.MustCompile(`Error: No such container:path: .*:layer-1.txt`))
			})

			it("returns error on nonexistent layer", func() {
				img, err := remote.NewImage(
					repoName,
					authn.DefaultKeychain,
					remote.WithPreviousImage(prevImageName),
					remote.FromBaseImage("busybox"),
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
			var tarPath string

			it.Before(func() {
				baseImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.SetLabel("mykey", "oldValue"))
				h.AssertNil(t, baseImage.Save())

				tarFile, err := ioutil.TempFile("", "add-layer-test")
				h.AssertNil(t, err)
				defer tarFile.Close()

				tarPath, err = h.CreateSingleFileTar("/new-layer.txt", "new-layer")
				h.AssertNil(t, err)
			})

			it.After(func() {
				h.AssertNil(t, os.Remove(tarPath))
			})

			it("can be pulled by digest", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				err = img.SetLabel("mykey", "newValue")
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())

				identifier, err := img.Identifier()
				h.AssertNil(t, err)

				// After Pull
				label := remoteLabel(t, dockerClient, identifier.String(), "mykey")
				h.AssertEq(t, "newValue", label)
			})

			it("zeroes all times and client specific fields", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, img.AddLayer(tarPath))

				h.AssertNil(t, img.Save())

				h.AssertNil(t, h.PullImage(dockerClient, repoName))
				defer h.DockerRmi(dockerClient, repoName)
				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
				h.AssertNil(t, err)

				h.AssertEq(t, inspect.Created, imgutil.NormalizedDateTime.Format(time.RFC3339))
				h.AssertEq(t, inspect.Container, "")
				h.AssertEq(t, inspect.DockerVersion, "")

				history, err := dockerClient.ImageHistory(context.TODO(), repoName)
				h.AssertNil(t, err)
				h.AssertEq(t, len(history), len(inspect.RootFS.Layers))
				for _, item := range history {
					h.AssertEq(t, item.Created, imgutil.NormalizedDateTime.Unix())
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

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, successfulRepoNames...))
			})

			it("saves to multiple names", func() {
				image, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, image.Save(additionalRepoNames...))
				for _, n := range successfulRepoNames {
					h.AssertNil(t, h.PullImage(dockerClient, n))
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
						h.AssertNil(t, h.PullImage(dockerClient, n))
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
				h.AssertNil(t, origImage.SetLabel("repo_name_for_randomisation", repoName))
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
				var err error

				origImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)
				h.AssertNil(t, origImage.SetLabel("repo_name_for_randomisation", repoName))
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

func manifestLayers(t *testing.T, repoName string) []string {
	t.Helper()

	arr := strings.SplitN(repoName, "/", 2)
	if len(arr) != 2 {
		t.Fatalf("expected repoName to have 1 slash (remote test registry): '%s'", repoName)
	}

	url := "http://" + arr[0] + "/v2/" + arr[1] + "/manifests/latest"
	req, err := http.NewRequest("GET", url, nil)
	h.AssertNil(t, err)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := http.DefaultClient.Do(req)
	h.AssertNil(t, err)
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("HTTP Status was bad: %s => %d", url, resp.StatusCode)
	}

	var manifest struct {
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	json.NewDecoder(resp.Body).Decode(&manifest)
	h.AssertNil(t, err)

	outSlice := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		outSlice = append(outSlice, layer.Digest)
	}

	return outSlice
}

func remoteLabel(t *testing.T, dockerCli client.CommonAPIClient, repoName, label string) string {
	t.Helper()

	h.AssertNil(t, h.PullImage(dockerCli, repoName))
	defer func() { h.AssertNil(t, h.DockerRmi(dockerCli, repoName)) }()
	inspect, _, err := dockerCli.ImageInspectWithRaw(context.TODO(), repoName)
	h.AssertNil(t, err)
	return inspect.Config.Labels[label]
}
