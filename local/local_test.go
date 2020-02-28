package local_test

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/local"
	h "github.com/buildpacks/imgutil/testhelpers"
)

var (
	localTestRegistry *h.DockerRegistry
)

func TestLocal(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())

	localTestRegistry = h.NewDockerRegistry()
	localTestRegistry.Start(t)
	defer localTestRegistry.Stop(t)

	spec.Run(t, "LocalImage", testLocalImage, spec.Sequential(), spec.Report(report.Terminal{}))
}

func newTestImageName(suffixOpt ...string) string {
	suffix := ":latest"
	if len(suffixOpt) == 1 {
		suffix = suffixOpt[0]
	}
	return fmt.Sprintf("%s:%s/pack-image-test-%s%s", localTestRegistry.Host, localTestRegistry.Port, h.RandString(10), suffix)
}

func testLocalImage(t *testing.T, when spec.G, it spec.S) {
	var (
		dockerClient          client.CommonAPIClient
		daemonInfo            types.Info
		runnableBaseImageName string
	)

	it.Before(func() {
		var err error
		dockerClient = h.DockerCli(t)
		h.AssertNil(t, err)

		daemonInfo, err = dockerClient.Info(context.TODO())
		h.AssertNil(t, err)

		runnableBaseImageName = "busybox@sha256:915f390a8912e16d4beb8689720a17348f3f6d1a7b659697df850ab625ea29d5"
		if daemonInfo.OSType == "windows" {
			runnableBaseImageName = "mcr.microsoft.com/windows/nanoserver@sha256:06281772b6a561411d4b338820d94ab1028fdeb076c85350bbc01e80c4bfa2b4"
		}

		h.PullImage(dockerClient, runnableBaseImageName)
	})

	when("#NewImage", func() {
		when("no base image is given", func() {
			it("returns an empty image", func() {
				_, err := local.NewImage(newTestImageName(), dockerClient)
				h.AssertNil(t, err)
			})

			it("sets sensible defaults from daemon for all required fields", func() {
				// os, architecture, and rootfs are required per https://github.com/opencontainers/image-spec/blob/master/config.md
				img, err := local.NewImage(newTestImageName(), dockerClient)
				h.AssertNil(t, err)
				h.AssertNil(t, img.Save())

				defer h.DockerRmi(dockerClient, img.Name())
				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), img.Name())
				h.AssertNil(t, err)

				// os, architecture come from daemon
				h.AssertEq(t, inspect.Os, daemonInfo.OSType)
				h.AssertEq(t, inspect.OsVersion, daemonInfo.OSVersion)
				h.AssertEq(t, inspect.Architecture, "amd64")
				h.AssertEq(t, inspect.RootFS.Type, "layers")
			})
		})

		when("#FromBaseImage", func() {
			when("base image exists", func() {
				var repoName = newTestImageName()

				it.After(func() {
					h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
				})

				it("returns the local image", func() {
					baseImage, err := local.NewImage(repoName, dockerClient)
					h.AssertNil(t, err)

					h.AssertNil(t, baseImage.SetLabel("some.label", "some.value"))
					h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
					h.AssertNil(t, baseImage.Save())

					localImage, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
					h.AssertNil(t, err)

					labelValue, err := localImage.Label("some.label")
					h.AssertNil(t, err)
					h.AssertEq(t, labelValue, "some.value")
				})
			})

			when("base image does not exist", func() {
				it("doesn't error", func() {
					_, err := local.NewImage(
						newTestImageName(),
						dockerClient,
						local.FromBaseImage("some-bad-repo-name"),
					)

					h.AssertNil(t, err)
				})
			})

			when("base image and daemon os/architecture match", func() {
				it("uses the base image architecture/OS", func() {
					img, err := local.NewImage(newTestImageName(), dockerClient, local.FromBaseImage(runnableBaseImageName))
					h.AssertNil(t, err)
					h.AssertNil(t, img.Save())
					defer h.DockerRmi(dockerClient, img.Name())

					imgOS, err := img.OS()
					h.AssertNil(t, err)
					h.AssertEq(t, imgOS, daemonInfo.OSType)

					inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), img.Name())
					h.AssertNil(t, err)
					h.AssertEq(t, inspect.Os, daemonInfo.OSType)
					h.AssertEq(t, inspect.OsVersion, daemonInfo.OSVersion)
					h.AssertEq(t, inspect.Architecture, "amd64")
					h.AssertEq(t, inspect.RootFS.Type, "layers")

					h.AssertEq(t, img.Found(), true)
				})
			})

			when("base image and daemon architecture do not match", func() {
				it("uses the base image architecture", func() {
					armBaseImageName := "arm64v8/busybox@sha256:50edf1d080946c6a76989d1c3b0e753b62f7d9b5f5e66e88bef23ebbd1e9709c"
					expectedArmArch := "arm64"
					if daemonInfo.OSType == "windows" {
						// this nanoserver windows/arm image exists and pulls. Not sure whether it works for anything else.
						armBaseImageName = "mcr.microsoft.com/windows/nanoserver@sha256:29e2270953589a12de7a77a7e77d39e3b3e9cdfd243c922b3b8a63e2d8a71026"
						expectedArmArch = "arm"
					}

					h.PullImage(dockerClient, armBaseImageName)

					img, err := local.NewImage(newTestImageName(), dockerClient, local.FromBaseImage(armBaseImageName))
					h.AssertNil(t, err)
					h.AssertNil(t, img.Save())
					defer h.DockerRmi(dockerClient, img.Name())

					imgArch, err := img.Architecture()
					h.AssertNil(t, err)

					h.AssertEq(t, imgArch, expectedArmArch)
				})
			})
		})

		when("#WithPreviousImage", func() {
			when("previous image does not exist", func() {
				it("doesn't error", func() {
					_, err := local.NewImage(
						runnableBaseImageName,
						dockerClient,
						local.WithPreviousImage("some-bad-repo-name"),
					)

					h.AssertNil(t, err)
				})
			})
		})
	})

	when("#Label", func() {
		when("image exists", func() {
			var repoName = newTestImageName()

			it.Before(func() {
				baseImage, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("mykey", "myvalue"))
				h.AssertNil(t, baseImage.SetLabel("other", "data"))
				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
			})

			it("returns the label value", func() {
				img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				label, err := img.Label("mykey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "myvalue")
			})

			it("returns an empty string for a missing label", func() {
				img, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				label, err := img.Label("missing-label")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "")
			})
		})

		when("image NOT exists", func() {
			it("returns an empty string", func() {
				img, err := local.NewImage(newTestImageName(), dockerClient)
				h.AssertNil(t, err)

				label, err := img.Label("some-label")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "")
			})
		})
	})

	when("#Env", func() {
		when("image exists", func() {
			var repoName = newTestImageName()

			it.Before(func() {
				baseImage, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetEnv("MY_VAR", "my_val"))
				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
			})

			it("returns the label value", func() {
				img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				val, err := img.Env("MY_VAR")
				h.AssertNil(t, err)
				h.AssertEq(t, val, "my_val")
			})

			it("returns an empty string for a missing label", func() {
				img, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				val, err := img.Env("MISSING_VAR")
				h.AssertNil(t, err)
				h.AssertEq(t, val, "")
			})
		})

		when("image NOT exists", func() {
			it("returns an empty string", func() {
				img, err := local.NewImage(newTestImageName(), dockerClient)
				h.AssertNil(t, err)

				val, err := img.Env("SOME_VAR")
				h.AssertNil(t, err)
				h.AssertEq(t, val, "")
			})
		})
	})

	when("#Name", func() {
		it("always returns the original name", func() {
			var repoName = newTestImageName()

			img, err := local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)

			h.AssertEq(t, img.Name(), repoName)
		})
	})

	when("#CreatedAt", func() {
		it("returns the containers created at time", func() {
			img, err := local.NewImage(newTestImageName(), dockerClient, local.FromBaseImage(runnableBaseImageName))
			h.AssertNil(t, err)

			// based on static base image refs
			expectedTime := time.Date(2018, 10, 2, 17, 19, 34, 239926273, time.UTC)
			if daemonInfo.OSType == "windows" {
				expectedTime = time.Date(2020, 02, 16, 01, 25, 57, 339000000, time.UTC)
			}

			createdTime, err := img.CreatedAt()

			h.AssertNil(t, err)
			h.AssertEq(t, createdTime, expectedTime)
		})
	})

	when("#Identifier", func() {
		var (
			repoName = newTestImageName()
		)

		it.Before(func() {
			baseImage, err := local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)

			h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
			h.AssertNil(t, baseImage.Save())
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
		})

		it("returns an Docker Image ID type identifier", func() {
			img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
			h.AssertNil(t, err)

			id, err := img.Identifier()
			h.AssertNil(t, err)

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), id.String())
			h.AssertNil(t, err)
			label := inspect.Config.Labels["repo_name_for_randomisation"]
			h.AssertEq(t, strings.TrimSpace(label), repoName)
		})

		when("the image has been modified and saved", func() {
			var origID string

			it.Before(func() {
				origID = h.ImageID(t, repoName)
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, origID))
			})

			it("returns the new image ID", func() {
				img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				h.AssertNil(t, img.SetLabel("new", "label"))

				h.AssertNil(t, img.Save())
				h.AssertNil(t, err)

				id, err := img.Identifier()
				h.AssertNil(t, err)

				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), id.String())
				h.AssertNil(t, err)

				label := inspect.Config.Labels["new"]
				h.AssertEq(t, strings.TrimSpace(label), "label")
			})
		})
	})

	when("#SetLabel", func() {
		var (
			img      imgutil.Image
			origID   string
			repoName = newTestImageName()
		)

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, origID))
		})

		when("image has labels", func() {
			it.Before(func() {
				var err error
				baseImage, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("some-key", "some-value"))
				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())

				img, err = local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				origID = h.ImageID(t, repoName)
			})

			it("sets label and saves label to docker daemon", func() {
				h.AssertNil(t, img.SetLabel("somekey", "new-val"))

				label, err := img.Label("somekey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "new-val")

				h.AssertNil(t, img.Save())

				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
				h.AssertNil(t, err)
				label = inspect.Config.Labels["somekey"]
				h.AssertEq(t, strings.TrimSpace(label), "new-val")
			})
		})

		when("no labels exists", func() {
			it.Before(func() {
				var err error
				baseImage, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetCmd("/usr/bin/run"))
				h.AssertNil(t, baseImage.Save())

				img, err = local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				origID = h.ImageID(t, repoName)
			})

			it("sets label and saves label to docker daemon", func() {
				h.AssertNil(t, img.SetLabel("somekey", "new-val"))

				label, err := img.Label("somekey")
				h.AssertNil(t, err)
				h.AssertEq(t, label, "new-val")

				h.AssertNil(t, img.Save())

				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
				h.AssertNil(t, err)

				label = inspect.Config.Labels["somekey"]
				h.AssertEq(t, strings.TrimSpace(label), "new-val")
			})
		})
	})

	when("#SetEnv", func() {
		var (
			img      imgutil.Image
			origID   string
			repoName = newTestImageName()
		)
		it.Before(func() {
			var err error
			baseImage, err := local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)

			h.AssertNil(t, baseImage.SetLabel("some-key", "some-value"))
			h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
			h.AssertNil(t, baseImage.Save())

			img, err = local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)
			origID = h.ImageID(t, repoName)
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, origID))
		})

		it("sets the environment", func() {
			err := img.SetEnv("ENV_KEY", "ENV_VAL")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertContains(t, inspect.Config.Env, "ENV_KEY=ENV_VAL")
		})
	})

	when("#SetWorkingDir", func() {
		var (
			img      imgutil.Image
			origID   string
			repoName = newTestImageName()
		)
		it.Before(func() {
			var err error
			baseImage, err := local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)

			h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
			h.AssertNil(t, baseImage.Save())

			img, err = local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)
			origID = h.ImageID(t, repoName)
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, origID))
		})

		it("sets the environment", func() {
			err := img.SetWorkingDir("/some/work/dir")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertEq(t, inspect.Config.WorkingDir, "/some/work/dir")
		})
	})

	when("#SetEntrypoint", func() {
		var (
			img      imgutil.Image
			origID   string
			repoName = newTestImageName()
		)
		it.Before(func() {
			var err error
			baseImage, err := local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)

			h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
			h.AssertNil(t, baseImage.Save())

			img, err = local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)
			origID = h.ImageID(t, repoName)
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, origID))
		})

		it("sets the entrypoint", func() {
			err := img.SetEntrypoint("some", "entrypoint")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertEq(t, []string(inspect.Config.Entrypoint), []string{"some", "entrypoint"})
		})
	})

	when("#SetCmd", func() {
		var (
			img      imgutil.Image
			origID   string
			repoName = newTestImageName()
		)

		it.Before(func() {
			var err error
			baseImage, err := local.NewImage(repoName, dockerClient)
			h.AssertNil(t, err)

			h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
			h.AssertNil(t, baseImage.Save())

			img, err = local.NewImage(repoName, dockerClient)
			origID = h.ImageID(t, repoName)
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, origID))
		})

		it("sets the cmd", func() {
			err := img.SetCmd("some", "cmd")
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertEq(t, []string(inspect.Config.Cmd), []string{"some", "cmd"})
		})
	})

	when("#Rebase", func() {
		when("image exists", func() {
			var (
				oldBase, oldTopLayer, newBase, origID string
				origNumLayers                         int
				repoName                              = newTestImageName()
			)

			it.Before(func() {
				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer wg.Done()
					newBase = newTestImageName(":new-base")
					h.CreateImageOnLocal(t, dockerClient, newBase, fmt.Sprintf(`
						FROM %s
						LABEL repo_name_for_randomisation=%s
						RUN echo new-base > base.txt
						RUN echo text-new-base > otherfile.txt
					`, runnableBaseImageName, newBase), nil)
				}()

				oldBase = newTestImageName(":old-base")
				h.CreateImageOnLocal(t, dockerClient, oldBase, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-base > base.txt
					RUN echo text-old-base > otherfile.txt
				`, runnableBaseImageName, oldBase), nil)
				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), oldBase)
				h.AssertNil(t, err)
				oldTopLayer = inspect.RootFS.Layers[len(inspect.RootFS.Layers)-1]

				h.CreateImageOnLocal(t, dockerClient, repoName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo text-from-image > myimage.txt
					RUN echo text-from-image > myimage2.txt
				`, oldBase, repoName), nil)
				inspect, _, err = dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
				h.AssertNil(t, err)
				origNumLayers = len(inspect.RootFS.Layers)
				origID = inspect.ID

				wg.Wait()
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, repoName, oldBase, newBase, origID))
			})

			it("switches the base", func() {
				// Before
				txt, err := h.CopySingleFileFromImage(dockerClient, repoName, "base.txt")
				h.AssertNil(t, err)
				h.AssertMatch(t, txt, regexp.MustCompile(`old-base[ \r\n]*$`))

				// Run rebase
				img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)
				newBaseImg, err := local.NewImage(newBase, dockerClient, local.FromBaseImage(newBase))
				h.AssertNil(t, err)
				err = img.Rebase(oldTopLayer, newBaseImg)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())

				// After
				expected := map[string]string{
					"base.txt":      "new-base",
					"otherfile.txt": "text-new-base",
					"myimage.txt":   "text-from-image",
					"myimage2.txt":  "text-from-image",
				}
				ctr, err := dockerClient.ContainerCreate(context.Background(), &container.Config{Image: repoName}, &container.HostConfig{}, nil, "")
				defer dockerClient.ContainerRemove(context.Background(), ctr.ID, types.ContainerRemoveOptions{})
				for filename, expectedText := range expected {
					actualText, err := h.CopySingleFileFromContainer(dockerClient, ctr.ID, filename)
					h.AssertNil(t, err)
					h.AssertMatch(t, actualText, regexp.MustCompile(expectedText+`[ \r\n]*$`))
				}

				// Final Image should have same number of layers as initial image
				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
				h.AssertNil(t, err)
				numLayers := len(inspect.RootFS.Layers)
				h.AssertEq(t, numLayers, origNumLayers)
			})
		})
	})

	when("#TopLayer", func() {
		when("image exists", func() {
			it("returns the digest for the top layer (useful for rebasing)", func() {
				repoName := newTestImageName()

				h.CreateImageOnLocal(t, dockerClient, repoName, fmt.Sprintf(`
				FROM %s
				LABEL repo_name_for_randomisation=%s
				RUN echo old-base > base.txt
				RUN echo text-old-base > otherfile.txt
				`, runnableBaseImageName, repoName), nil)
				defer h.DockerRmi(dockerClient, repoName)

				inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
				h.AssertNil(t, err)
				expectedTopLayer := inspect.RootFS.Layers[len(inspect.RootFS.Layers)-1]

				img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				actualTopLayer, err := img.TopLayer()
				h.AssertNil(t, err)

				h.AssertEq(t, actualTopLayer, expectedTopLayer)
			})
		})

		when("image has no layers", func() {
			it("returns error", func() {
				img, err := local.NewImage(newTestImageName(), dockerClient)
				h.AssertNil(t, err)

				_, err = img.TopLayer()
				h.AssertError(t, err, "has no layers")
			})
		})
	})

	when("#AddLayer", func() {
		var (
			tarPath  string
			origID   string
			repoName = newTestImageName()
		)

		it.Before(func() {
			tr, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer", daemonInfo.OSType)
			h.AssertNil(t, err)
			tarFile, err := ioutil.TempFile("", "add-layer-test")
			h.AssertNil(t, err)
			defer tarFile.Close()
			_, err = io.Copy(tarFile, tr)
			h.AssertNil(t, err)
			tarPath = tarFile.Name()
		})

		it.After(func() {
			h.AssertNil(t, os.Remove(tarPath))
		})

		when("empty image", func() {
			it("fails to save", func() {
				if daemonInfo.OSType == "windows" {
					t.Skip("not yet implemented on windows: fails to Save without base-image")
					return
				}

				img, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				err = img.AddLayer(tarPath)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())

				defer h.DockerRmi(dockerClient, repoName)
			})
		})

		when("new image from base-image", func() {
			it("appends a layer", func() {
				img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(runnableBaseImageName))
				h.AssertNil(t, err)

				err = img.AddLayer(tarPath)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())
				defer h.DockerRmi(dockerClient, repoName)

				output, err := h.CopySingleFileFromImage(dockerClient, repoName, "new-layer.txt")
				h.AssertNil(t, err)
				h.AssertMatch(t, output, regexp.MustCompile(`new-layer[ \r\n]*$`))
			})
		})

		when("Dockerfile generated base-image", func() {
			it("appends a layer", func() {
				h.CreateImageOnLocal(t, dockerClient, repoName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-layer > old-layer.txt
				`, runnableBaseImageName, repoName), nil)

				img, err := local.NewImage(
					repoName, dockerClient,
					local.FromBaseImage(repoName),
					local.WithPreviousImage(repoName),
				)
				h.AssertNil(t, err)
				origID = h.ImageID(t, repoName)

				err = img.AddLayer(tarPath)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Save())
				defer h.DockerRmi(dockerClient, repoName, origID)

				output, err := h.CopySingleFileFromImage(dockerClient, repoName, "old-layer.txt")
				h.AssertNil(t, err)
				h.AssertMatch(t, output, regexp.MustCompile(`old-layer[ \r\n]*$`))

				output, err = h.CopySingleFileFromImage(dockerClient, repoName, "new-layer.txt")
				h.AssertNil(t, err)
				h.AssertMatch(t, output, regexp.MustCompile(`new-layer[ \r\n]*$`))
			})
		})
	})

	when("#AddLayerWithDiffID", func() {
		var (
			tarPath  string
			diffID   string
			img      imgutil.Image
			origID   string
			repoName = newTestImageName()
		)
		it.Before(func() {
			h.CreateImageOnLocal(t, dockerClient, repoName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-layer > old-layer.txt
				`, runnableBaseImageName, repoName), nil)
			tr, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer", daemonInfo.OSType)
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

			img, err = local.NewImage(
				repoName, dockerClient,
				local.FromBaseImage(repoName),
				local.WithPreviousImage(repoName),
			)
			h.AssertNil(t, err)
			origID = h.ImageID(t, repoName)
		})

		it.After(func() {
			err := os.Remove(tarPath)
			h.AssertNil(t, err)
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, origID))
		})

		it("appends a layer", func() {
			err := img.AddLayerWithDiffID(tarPath, diffID)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			output, err := h.CopySingleFileFromImage(dockerClient, repoName, "old-layer.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`old-layer[ \r\n]*$`))

			output, err = h.CopySingleFileFromImage(dockerClient, repoName, "new-layer.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`new-layer[ \r\n]*$`))
		})
	})

	when("#GetLayer", func() {
		when("the layer exists", func() {
			var repoName = newTestImageName()

			it.Before(func() {
				baseImage, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(runnableBaseImageName))
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))

				tr, err := h.CreateSingleFileTar("file.txt", "file-contents", daemonInfo.OSType)
				h.AssertNil(t, err)
				tarFile, err := ioutil.TempFile("", "get-layer-test")
				h.AssertNil(t, err)
				defer tarFile.Close()
				defer os.RemoveAll(tarFile.Name())

				_, err = io.Copy(tarFile, tr)
				h.AssertNil(t, err)
				tarPath := tarFile.Name()

				err = baseImage.AddLayer(tarPath)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.Save())
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
			})

			it("returns a layer tar", func() {
				img, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)
				topLayer, err := img.TopLayer()
				h.AssertNil(t, err)
				r, err := img.GetLayer(topLayer)
				h.AssertNil(t, err)
				tr := tar.NewReader(r)

				if daemonInfo.OSType == "windows" {
					_, err = tr.Next() //Files dir
					h.AssertNil(t, err)

					_, err = tr.Next() //Hives dir
					h.AssertNil(t, err)

					header, err := tr.Next()
					h.AssertNil(t, err)
					h.AssertEq(t, header.Name, "Files/file.txt")
				} else {
					header, err := tr.Next()
					h.AssertNil(t, err)
					h.AssertEq(t, header.Name, "file.txt")
				}

				contents := make([]byte, len("file-contents"), len("file-contents"))
				_, err = tr.Read(contents)
				if err != io.EOF {
					t.Fatalf("expected end of file: %x", err)
				}
				h.AssertEq(t, string(contents), "file-contents")
			})
		})

		when("the layer doesn't exist", func() {
			var repoName = newTestImageName()

			it.Before(func() {
				baseImage, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(runnableBaseImageName))
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())
			})

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, repoName))
			})

			it("returns an error", func() {
				img, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)
				_, err = img.GetLayer("not-exist")
				h.AssertError(
					t,
					err,
					fmt.Sprintf("image '%s' does not contain layer with diff ID 'not-exist'", repoName),
				)
			})
		})

		when("image does NOT exist", func() {
			it("returns error", func() {
				image, err := local.NewImage(newTestImageName(), dockerClient)
				h.AssertNil(t, err)

				readCloser, err := image.GetLayer(h.RandString(10))
				h.AssertNil(t, readCloser)
				h.AssertError(t, err, "reference does not exist")
			})
		})
	})

	when("#ReuseLayer", func() {
		var (
			layer1SHA string
			layer2SHA string
			img       imgutil.Image
			prevName  = newTestImageName()
			repoName  = newTestImageName()
		)
		it.Before(func() {
			var err error

			h.CreateImageOnLocal(t, dockerClient, prevName, fmt.Sprintf(`
					FROM %s
					LABEL repo_name_for_randomisation=%s
					RUN echo old-layer-1 > layer-1.txt
					RUN echo old-layer-2 > layer-2.txt
				`, runnableBaseImageName, prevName), nil)

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), prevName)
			h.AssertNil(t, err)

			layer1SHA = inspect.RootFS.Layers[len(inspect.RootFS.Layers)-2]
			layer2SHA = inspect.RootFS.Layers[len(inspect.RootFS.Layers)-1]

			img, err = local.NewImage(
				repoName,
				dockerClient,
				local.FromBaseImage(runnableBaseImageName),
				local.WithPreviousImage(prevName),
			)
			h.AssertNil(t, err)
		})

		it.After(func() {
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, prevName))
		})

		it("reuses a layer", func() {
			err := img.ReuseLayer(layer2SHA)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			output, err := h.CopySingleFileFromImage(dockerClient, repoName, "layer-2.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`old-layer-2[ \r\n]*$`))

			// Confirm layer-1.txt does not exist
			_, err = h.CopySingleFileFromImage(dockerClient, repoName, "layer-1.txt")
			h.AssertMatch(t, err.Error(), regexp.MustCompile(`Error: No such container:path: .*:layer-1.txt`))
		})

		it("does not download the old image if layers are directly above (performance)", func() {
			err := img.ReuseLayer(layer1SHA)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			output, err := h.CopySingleFileFromImage(dockerClient, repoName, "layer-1.txt")
			h.AssertNil(t, err)
			h.AssertMatch(t, output, regexp.MustCompile(`old-layer-1[ \r\n]*$`))

			// Confirm layer-2.txt does not exist
			_, err = h.CopySingleFileFromImage(dockerClient, repoName, "layer-2.txt")
			h.AssertMatch(t, err.Error(), regexp.MustCompile(`Error: No such container:path: .*:layer-2.txt`))
		})
	})

	when("#Save", func() {
		var (
			img      imgutil.Image
			origID   string
			tarPath  string
			repoName = newTestImageName()
		)

		it.Before(func() {
			var err error

			baseImage, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(runnableBaseImageName))
			h.AssertNil(t, err)

			h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
			h.AssertNil(t, baseImage.SetLabel("mykey", "oldValue"))
			h.AssertNil(t, baseImage.Save())

			origID = h.ImageID(t, repoName)

			img, err = local.NewImage(repoName, dockerClient, local.FromBaseImage(runnableBaseImageName))
			h.AssertNil(t, err)

			tr, err := h.CreateSingleFileTar("/new-layer.txt", "new-layer-content", daemonInfo.OSType)
			h.AssertNil(t, err)
			tarFile, err := ioutil.TempFile("", "save-test")
			h.AssertNil(t, err)
			defer tarFile.Close()
			_, err = io.Copy(tarFile, tr)
			h.AssertNil(t, err)
			tarPath = tarFile.Name()
		})

		it.After(func() {
			os.Remove(tarPath)
			h.AssertNil(t, h.DockerRmi(dockerClient, repoName, origID))
		})

		it("saved image overrides image with new ID", func() {
			err := img.SetLabel("mykey", "newValue")
			h.AssertNil(t, err)

			err = img.AddLayer(tarPath)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			identifier, err := img.Identifier()
			h.AssertNil(t, err)

			h.AssertEq(t, origID != identifier.String(), true)

			newImageID := h.ImageID(t, repoName)
			h.AssertNotEq(t, origID, newImageID)

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), identifier.String())
			h.AssertNil(t, err)
			label := inspect.Config.Labels["mykey"]
			h.AssertEq(t, strings.TrimSpace(label), "newValue")
		})

		it("zeroes times and client specific fields", func() {
			err := img.SetLabel("mykey", "newValue")
			h.AssertNil(t, err)

			err = img.AddLayer(tarPath)
			h.AssertNil(t, err)

			h.AssertNil(t, img.Save())

			inspect, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), repoName)
			h.AssertNil(t, err)

			h.AssertEq(t, inspect.Created, imgutil.NormalizedDateTime.Format(time.RFC3339))
			h.AssertEq(t, inspect.Container, "")
			h.AssertEq(t, inspect.DockerVersion, "")

			history, err := dockerClient.ImageHistory(context.TODO(), repoName)
			h.AssertNil(t, err)
			h.AssertEq(t, len(history), len(inspect.RootFS.Layers))
			for i, _ := range inspect.RootFS.Layers {
				h.AssertEq(t, history[i].Created, imgutil.NormalizedDateTime.Unix())
			}
		})

		when("additional names are provided", func() {
			var (
				additionalRepoNames = []string{
					newTestImageName(":"+h.RandString(5)),
					newTestImageName(),
					newTestImageName(),
				}
				successfulRepoNames = append([]string{repoName}, additionalRepoNames...)
			)

			it.After(func() {
				h.AssertNil(t, h.DockerRmi(dockerClient, additionalRepoNames...))
			})

			it("saves to multiple names", func() {
				h.AssertNil(t, img.Save(additionalRepoNames...))

				for _, n := range successfulRepoNames {
					_, _, err := dockerClient.ImageInspectWithRaw(context.TODO(), n)
					h.AssertNil(t, err)
				}
			})

			when("a single image name fails", func() {
				it("returns results with errors for those that failed", func() {
					failingName := newTestImageName() + ":🧨"

					err := img.Save(append([]string{failingName}, additionalRepoNames...)...)
					h.AssertError(t, err, fmt.Sprintf("failed to write image to the following tags: [%s:", failingName))

					saveErr, ok := err.(imgutil.SaveError)
					h.AssertEq(t, ok, true)
					h.AssertEq(t, len(saveErr.Errors), 1)
					h.AssertEq(t, saveErr.Errors[0].ImageName, failingName)
					h.AssertError(t, saveErr.Errors[0].Cause, "invalid reference format")

					for _, n := range successfulRepoNames {
						_, _, err = dockerClient.ImageInspectWithRaw(context.TODO(), n)
						h.AssertNil(t, err)
					}
				})
			})
		})
	})

	when("#Found", func() {
		when("it exists", func() {
			var repoName = newTestImageName()

			it.Before(func() {
				baseImage, err := local.NewImage(repoName, dockerClient)
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())
			})

			it.After(func() {
				h.DockerRmi(dockerClient, repoName)
			})

			it("returns true, nil", func() {
				image, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				h.AssertEq(t, image.Found(), true)
			})
		})

		when("it does not exist", func() {
			it("returns false, nil", func() {
				image, err := local.NewImage(newTestImageName(), dockerClient)
				h.AssertNil(t, err)

				h.AssertEq(t, image.Found(), false)
			})
		})
	})

	when("#Delete", func() {
		when("the image does not exist", func() {
			it("should not error", func() {
				img, err := local.NewImage("image-does-not-exist", dockerClient)
				h.AssertNil(t, err)

				h.AssertNil(t, img.Delete())
			})
		})

		when("the image does exist", func() {
			var (
				origImg  imgutil.Image
				origID   string
				repoName = newTestImageName()
			)

			it.Before(func() {
				var err error
				baseImage, err := local.NewImage(repoName, dockerClient, local.FromBaseImage(runnableBaseImageName))
				h.AssertNil(t, err)

				h.AssertNil(t, baseImage.SetLabel("mykey", "oldValue"))
				h.AssertNil(t, baseImage.SetLabel("repo_name_for_randomisation", repoName))
				h.AssertNil(t, baseImage.Save())

				origImg, err = local.NewImage(repoName, dockerClient, local.FromBaseImage(repoName))
				h.AssertNil(t, err)

				origID = h.ImageID(t, repoName)
			})

			it("should delete the image", func() {
				h.AssertEq(t, origImg.Found(), true)

				h.AssertNil(t, origImg.Delete())

				img, err := local.NewImage(origID, dockerClient)
				h.AssertNil(t, err)

				h.AssertEq(t, img.Found(), false)
			})

			when("the image has been re-tagged", func() {
				const newTag = "different-tag"

				it.Before(func() {
					h.AssertNil(t, dockerClient.ImageTag(context.TODO(), origImg.Name(), newTag))

					_, err := dockerClient.ImageRemove(context.TODO(), origImg.Name(), types.ImageRemoveOptions{})
					h.AssertNil(t, err)
				})

				it("should delete the image", func() {
					h.AssertEq(t, origImg.Found(), true)

					h.AssertNil(t, origImg.Delete())

					origImg, err := local.NewImage(newTag, dockerClient)
					h.AssertNil(t, err)

					h.AssertEq(t, origImg.Found(), false)
				})
			})
		})
	})
}
