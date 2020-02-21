package remote_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	ggcrname "github.com/google/go-containerregistry/pkg/name"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sync"
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

var localTestRegistry *h.DockerRegistry

func newTestImageName() string {
	return fmt.Sprintf("%s:%s/pack-image-test-%s", localTestRegistry.Host, localTestRegistry.Port, h.RandString(10))
}

func TestRemoteImage(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())

	localTestRegistry = h.NewDockerRegistry()
	localTestRegistry.Start(t)
	defer localTestRegistry.Stop(t)

	spec.Run(t, "RemoteImage", testRemoteImage, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testRemoteImage(t *testing.T, when spec.G, it spec.S) {
	var (
		repoName              string
		dockerClient          client.CommonAPIClient
		runnableBaseImageName string
		daemonOSType          string
	)

	it.Before(func() {
		var err error
		dockerClient = h.DockerCli(t)
		h.AssertNil(t, err)
		repoName = newTestImageName()

		daemonInfo, err := dockerClient.Info(context.TODO())
		h.AssertNil(t, err)

		daemonOSType = daemonInfo.OSType

		runnableBaseImageName = "amd64/busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"
		if daemonOSType == "windows" {
			runnableBaseImageName = "mcr.microsoft.com/windows/nanoserver@sha256:06281772b6a561411d4b338820d94ab1028fdeb076c85350bbc01e80c4bfa2b4"
		}

		h.PullImage(dockerClient, runnableBaseImageName)
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
				h.AssertEq(t, os, "linux")

				arch, err := img.Architecture()
				h.AssertEq(t, arch, "amd64")

				topLayer, err := img.TopLayer()
				h.AssertEq(t, topLayer, "")
			})
		})

		when("#WithDefaultPlatform", func() {
			when("no base image is given", func() {
				it("sets os and architecture", func() {
					img, err := remote.NewImage(
						newTestImageName(),
						authn.DefaultKeychain,
						remote.WithDefaultPlatform(
							ggcrv1.Platform{OS: "windows", Architecture: "arm"}),
					)
					h.AssertNil(t, err)
					h.AssertNil(t, img.Save())

					os, err := img.OS()
					h.AssertEq(t, os, "windows")

					arch, err := img.Architecture()
					h.AssertEq(t, arch, "arm")

					topLayer, err := img.TopLayer()
					h.AssertEq(t, topLayer, "")
				})
			})
		})

		when("#FromRemoteBaseImage", func() {
			when("base image exists", func() {
				it("sets the initial state from a linux/amd64 base image", func() {
					baseImageName := "amd64/busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"
					existingLayerSha := "sha256:8a788232037eaf17794408ff3df6b922a1aedf9ef8de36afdae3ed0b0381907b"

					img, err := remote.NewImage(
						repoName,
						authn.DefaultKeychain,
						remote.FromBaseImage(baseImageName),
					)
					h.AssertNil(t, err)

					os, err := img.OS()
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := img.Architecture()
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					readCloser, err := img.GetLayer(existingLayerSha)
					h.AssertNil(t, err)
					defer readCloser.Close()
				})

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

						arch, err := img.Architecture()
						h.AssertNil(t, err)
						h.AssertEq(t, arch, "amd64")

						readCloser, err := img.GetLayer(existingLayerSha)
						h.AssertNil(t, err)
						defer readCloser.Close()
					})

					when("and WithDefaultPlatform is set to windows/amd64", func() {
						it("returns a base image matching the runtime GOOS/GOARCH", func() {
							manifestListName := "golang:1.13.8"
							existingLayerSha := "sha256:daf52c42c7d1d7b71581d309f4150b06fa5da3a0f53616f48bfdbdbb0e4fd171"

							img, err := remote.NewImage(
								repoName,
								authn.DefaultKeychain,
								remote.WithDefaultPlatform(
									ggcrv1.Platform{OS: "windows", Architecture: "amd64", OSVersion: "10.0.17763.1040"}),
								remote.FromBaseImage(manifestListName),
							)
							h.AssertNil(t, err)

							os, err := img.OS()
							h.AssertNil(t, err)
							h.AssertEq(t, os, "windows")

							arch, err := img.Architecture()
							h.AssertNil(t, err)
							h.AssertEq(t, arch, "amd64")

							readCloser, err := img.GetLayer(existingLayerSha)
							h.AssertNil(t, err)
							defer readCloser.Close()
						})
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
			var img imgutil.Image
			it.Before(func() {
				var err error
				baseImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("mykey", "myvalue"))
				h.AssertNil(t, baseImage.SetLabel("other", "data"))
				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())

				img, err = remote.NewImage(
					repoName, authn.DefaultKeychain,
					remote.FromBaseImage(repoName),
				)
				h.AssertNil(t, err)
			})

			it("returns the label value", func() {
				label, err := img.Label("mykey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "myvalue")
			})

			it("returns an empty string for a missing label", func() {
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
			var (
				img imgutil.Image
			)
			it.Before(func() {
				var err error
				baseImage, err := remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetEnv("MY_VAR", "my_val"))
				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())

				img, err = remote.NewImage(
					repoName,
					authn.DefaultKeychain,
					remote.FromBaseImage(repoName),
				)
				h.AssertNil(t, err)
			})

			it("returns the label value", func() {
				val, err := img.Env("MY_VAR")
				h.AssertNil(t, err)
				h.AssertEq(t, val, "my_val")
			})

			it("returns an empty string for a missing label", func() {
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
		var img imgutil.Image

		it.Before(func() {
			var err error
			img, err = remote.NewImage(
				repoName+":some-tag",
				authn.DefaultKeychain,
				remote.FromBaseImage("busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"),
			)
			h.AssertNil(t, err)
		})

		it("returns a digest reference", func() {
			identifier, err := img.Identifier()
			h.AssertNil(t, err)
			h.AssertEq(t, identifier.String(), repoName+"@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5")
		})

		it("accurately parses the reference for an image with a sha", func() {
			var err error
			img, err = remote.NewImage(
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
				h.AssertNil(t, img.SetLabel("new", "label"))

				h.AssertNil(t, img.Save())

				id, err := img.Identifier()
				h.AssertNil(t, err)

				testImg, err := remote.NewImage(
					"test",
					authn.DefaultKeychain,
					remote.FromBaseImage(id.String()),
				)

				remoteLabel, err := testImg.Label("new")
				h.AssertNil(t, err)

				h.AssertEq(t, remoteLabel, "label")
			})
		})
	})

	when("#SetLabel", func() {
		var img imgutil.Image
		when("image exists", func() {
			it.Before(func() {
				var err error
				img, err = remote.NewImage(repoName, authn.DefaultKeychain)
				h.AssertNil(t, err)
			})

			it("sets label on img object", func() {
				h.AssertNil(t, img.SetLabel("mykey", "new-val"))
				label, err := img.Label("mykey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "new-val")
			})

			it("saves label", func() {
				h.AssertNil(t, img.SetLabel("mykey", "new-val"))

				h.AssertNil(t, img.Save())

				testImg, err := remote.NewImage(
					"test",
					authn.DefaultKeychain,
					remote.FromBaseImage(repoName),
				)

				remoteLabel, err := testImg.Label("mykey")
				h.AssertNil(t, err)

				h.AssertEq(t, remoteLabel, "new-val")
			})
		})
	})

	when("#SetEnv", func() {
		var (
			img imgutil.Image
		)
		it.Before(func() {
			var err error
			img, err = remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)
		})

		it("sets the environment", func() {
			err := img.SetEnv("ENV_KEY", "ENV_VAL")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			testImg, err := remote.NewImage(
				"test",
				authn.DefaultKeychain,
				remote.FromBaseImage(repoName),
			)

			remoteEnv, err := testImg.Env("ENV_KEY")
			h.AssertEq(t, remoteEnv, "ENV_VAL")
		})
	})

	when("#SetWorkingDir", func() {
		var (
			img imgutil.Image
		)
		it.Before(func() {
			var err error
			img, err = remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)
		})

		it("sets the environment", func() {
			err := img.SetWorkingDir("/some/work/dir")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			config := manifestImageConfig(t, repoName, daemonOSType)

			h.AssertEq(t, config.WorkingDir, "/some/work/dir")
		})
	})

	when("#SetEntrypoint", func() {
		var (
			img imgutil.Image
		)
		it.Before(func() {
			var err error
			img, err = remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)
		})

		it("sets the entrypoint", func() {
			err := img.SetEntrypoint("some", "entrypoint")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			config := manifestImageConfig(t, repoName, daemonOSType)

			h.AssertEq(t, []string(config.Entrypoint), []string{"some", "entrypoint"})
		})
	})

	when("#SetCmd", func() {
		var (
			img imgutil.Image
		)
		it.Before(func() {
			var err error
			img, err = remote.NewImage(repoName, authn.DefaultKeychain)
			h.AssertNil(t, err)
		})

		it("sets the cmd", func() {
			err := img.SetCmd("some", "cmd")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			config := manifestImageConfig(t, repoName, daemonOSType)

			h.AssertEq(t, config.Cmd, []string{"some", "cmd"})
		})
	})

	when.Pend("#Rebase", func() {
		when("image exists", func() {
			var oldBase, oldTopLayer, newBase string
			var oldBaseLayers, newBaseLayers, repoTopLayers []string
			it.Before(func() {
				var wg sync.WaitGroup
				wg.Add(1)

				newBase = fmt.Sprintf("%s:%s/pack-newbase-test-%s", localTestRegistry.Host, localTestRegistry.Port, h.RandString(10))
				go func() {
					defer wg.Done()
					h.CreateImageOnRemote(t, dockerClient, newBase, fmt.Sprintf(`
						FROM %s
						LABEL repo_name_for_randomisation=%s
						RUN echo new-base > base.txt
						RUN echo text-new-base > otherfile.txt
					`, runnableBaseImageName, repoName), nil)
					newBaseLayers = manifestLayers(t, newBase, daemonOSType)
				}()

				oldBase = fmt.Sprintf("%s:%s/pack-oldbase-test-%s", localTestRegistry.Host, localTestRegistry.Port, h.RandString(10))
				oldTopLayer = h.CreateImageOnRemote(t, dockerClient, oldBase, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-base > base.txt
					RUN echo text-old-base > otherfile.txt
				`, runnableBaseImageName, oldBase), nil)
				oldBaseLayers = manifestLayers(t, oldBase, daemonOSType)

				h.CreateImageOnRemote(t, dockerClient, repoName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo text-from-image-1 > myimage.txt
					RUN echo text-from-image-2 > myimage2.txt
				`, oldBase, repoName), nil)
				repoTopLayers = manifestLayers(t, repoName, daemonOSType)[len(oldBaseLayers):]

				wg.Wait()
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, oldBase))
			})

			it("switches the base", func() {
				// Before
				h.AssertEq(t,
					manifestLayers(t, repoName, daemonOSType),
					append(oldBaseLayers, repoTopLayers...),
				)

				// Run rebase
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
				h.AssertNil(t, err)
				newBaseImg, err := remote.NewImage(newBase, authn.DefaultKeychain, remote.FromBaseImage(newBase))
				h.AssertNil(t, err)
				err = img.Rebase(oldTopLayer, newBaseImg)
				h.AssertNil(t, err)
				h.AssertNil(t, img.Save())

				// After
				h.AssertEq(t,
					manifestLayers(t, repoName, daemonOSType),
					append(newBaseLayers, repoTopLayers...),
				)
			})
		})
	})

	when("#TopLayer", func() {
		when("image exists", func() {
			it("returns the digest for the top layer (useful for rebasing)", func() {
				expectedTopLayer := h.CreateImageOnRemote(t, dockerClient, repoName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-base > base.txt
					RUN echo text-old-base > otherfile.txt
				`, runnableBaseImageName, repoName), nil)

				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
				h.AssertNil(t, err)

				actualTopLayer, err := img.TopLayer()
				h.AssertNil(t, err)

				h.AssertEq(t, actualTopLayer, expectedTopLayer)
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
		var (
			tarPath string
			img     imgutil.Image
		)
		it.Before(func() {
			h.CreateImageOnRemote(t, dockerClient, repoName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-layer > old-layer.txt
				`, runnableBaseImageName, repoName), nil)

			tr, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer", daemonOSType)
			h.AssertNil(t, err)

			tarFile, err := ioutil.TempFile("", "add-layer-test")
			h.AssertNil(t, err)
			defer tarFile.Close()

			_, err = io.Copy(tarFile, tr)
			h.AssertNil(t, err)
			tarPath = tarFile.Name()

			img, err = remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
			h.AssertNil(t, err)
		})

		it.After(func() {
			h.AssertNil(t, os.Remove(tarPath))
		})

		it("appends a layer", func() {
			err := img.AddLayer(tarPath)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			// After Pull
			h.AssertNil(t, h.PullImage(dockerClient, repoName))

			output, err := h.CopySingleFileFromImage(dockerClient, repoName, "old-layer.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`old-layer[ \r\n]*$`))

			output, err = h.CopySingleFileFromImage(dockerClient, repoName, "new-layer.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`new-layer[ \r\n]*$`))
		})
	})

	when("#AddLayerWithDiffID", func() {
		var (
			tarPath string
			diffID  string
			img     imgutil.Image
		)

		it.Before(func() {
			h.CreateImageOnRemote(t, dockerClient, repoName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-layer > old-layer.txt
				`, runnableBaseImageName, repoName), nil)
			tr, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer", daemonOSType)
			h.AssertNil(t, err)

			tarFile, err := ioutil.TempFile("", "add-layer-test")
			h.AssertNil(t, err)
			defer tarFile.Close()
			hasher := sha256.New()
			mw := io.MultiWriter(tarFile, hasher)
			_, err = io.Copy(mw, tr)
			h.AssertNil(t, err)
			tarPath = tarFile.Name()
			diffID = "sha256:" + hex.EncodeToString(hasher.Sum(make([]byte, 0, hasher.Size())))

			img, err = remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(repoName))
			h.AssertNil(t, err)
		})

		it.After(func() {
			h.AssertNil(t, os.Remove(tarPath))
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
		})

		it("appends a layer", func() {
			err := img.AddLayerWithDiffID(tarPath, diffID)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			// After Pull
			h.AssertNil(t, h.PullImage(dockerClient, repoName))

			output, err := h.CopySingleFileFromImage(dockerClient, repoName, "old-layer.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`old-layer[ \r\n]*$`))

			output, err = h.CopySingleFileFromImage(dockerClient, repoName, "new-layer.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`new-layer[ \r\n]*$`))
		})
	})

	when("#ReuseLayer", func() {
		when("previous image", func() {
			var (
				layer2SHA     string
				img           imgutil.Image
				prevImageName string
			)

			it.Before(func() {
				var err error

				prevImageName = fmt.Sprintf("%s:%s/pack-image-test-%s", localTestRegistry.Host, localTestRegistry.Port, h.RandString(10))
				h.CreateImageOnRemote(t, dockerClient, prevImageName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-layer-1 > layer-1.txt
					RUN echo old-layer-2 > layer-2.txt
				`, runnableBaseImageName, repoName), nil)

				h.AssertNil(t, h.PullImage(dockerClient, prevImageName))
				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), prevImageName)
				h.AssertNil(t, err)

				layer2SHA = inspect.RootFS.Layers[len(inspect.RootFS.Layers)-1]

				img, err = remote.NewImage(
					repoName,
					authn.DefaultKeychain,
					remote.WithPreviousImage(prevImageName),
					remote.FromBaseImage(runnableBaseImageName),
				)
				h.AssertNil(t, err)
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, prevImageName))
			})

			it("reuses a layer", func() {
				err := img.ReuseLayer(layer2SHA)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())

				h.AssertNil(t, h.PullImage(dockerClient, repoName))
				defer h.DockerRmi(dockerClient, repoName)
				output, err := h.CopySingleFileFromImage(dockerClient, repoName, "layer-2.txt")
				h.AssertNil(t, err)
				h.AssertMatch(t, output, regexp.MustCompile(`old-layer-2[ \r\n]*$`))

				// Confirm layer-1.txt does not exist
				_, err = h.CopySingleFileFromImage(dockerClient, repoName, "layer-1.txt")
				h.AssertMatch(t, err.Error(), regexp.MustCompile(`Error: No such container:path: .*:layer-1.txt`))
			})

			it("returns error on nonexistent layer", func() {
				img.Rename(repoName)
				err := img.ReuseLayer("some-bad-sha")

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

				tr, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer", daemonOSType)
				h.AssertNil(t, err)

				_, err = io.Copy(tarFile, tr)
				h.AssertNil(t, err)
				tarPath = tarFile.Name()
			})

			it.After(func() {
				defer h.DockerRmi(dockerClient, repoName)
				os.Remove(tarPath)
			})

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

				remoteLabel, err := testImg.Label("mykey")
				h.AssertNil(t, err)

				h.AssertEq(t, remoteLabel, "newValue")
			})

			it("zeroes all times and client specific fields", func() {
				img, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(runnableBaseImageName))
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
				image, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(runnableBaseImageName))
				h.AssertNil(t, err)

				h.AssertNil(t, image.Save(additionalRepoNames...))
				for _, n := range successfulRepoNames {
					h.AssertNil(t, h.PullImage(dockerClient, n))
				}
			})

			when("a single image name fails", func() {
				it("returns results with errors for those that failed", func() {
					failingName := newTestImageName() + ":🧨"

					image, err := remote.NewImage(repoName, authn.DefaultKeychain, remote.FromBaseImage(runnableBaseImageName))
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
				h.AssertNil(t, origImage.Save())

				img, err = remote.NewImage(
					repoName, authn.DefaultKeychain,
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
				h.AssertError(t, img.Delete(), "Not Found")
			})
		})
	})
}

func manifestLayers(t *testing.T, repoName, imageOS string) []string {
	t.Helper()

	r, err := ggcrname.ParseReference(repoName, ggcrname.WeakValidation)
	h.AssertNil(t, err)

	gImg, err := ggcrremote.Image(
		r,
		ggcrremote.WithPlatform(ggcrv1.Platform{OS: imageOS}),
		ggcrremote.WithTransport(http.DefaultTransport),
	)
	h.AssertNil(t, err)

	gLayers, err := gImg.Layers()
	h.AssertNil(t, err)

	var manifestLayers []string
	for _, gLayer := range gLayers {
		diffID, err := gLayer.DiffID()
		h.AssertNil(t, err)

		manifestLayers = append(manifestLayers, diffID.String())
	}

	return manifestLayers
}

func manifestImageConfig(t *testing.T, repoName, imageOS string) ggcrv1.Config {
	t.Helper()

	r, err := ggcrname.ParseReference(repoName, ggcrname.WeakValidation)
	h.AssertNil(t, err)

	gImg, err := ggcrremote.Image(
		r,
		ggcrremote.WithPlatform(ggcrv1.Platform{OS: imageOS}),
		ggcrremote.WithTransport(http.DefaultTransport),
	)
	h.AssertNil(t, err)

	configFile, err := gImg.ConfigFile()
	h.AssertNil(t, err)

	return configFile.Config
}
