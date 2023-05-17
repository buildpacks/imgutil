package remote_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/remote"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func newTestIndexName(providedPrefix ...string) string {
	prefix := "pack-index-test"
	if len(providedPrefix) > 0 {
		prefix = providedPrefix[0]
	}

	return dockerRegistry.RepoName(prefix + "-" + h.RandString(10))
}

func TestIndex(t *testing.T) {

	dockerConfigDir, err := ioutil.TempDir("", "test.docker.config.dir")
	h.AssertNil(t, err)
	defer os.RemoveAll(dockerConfigDir)

	sharedRegistryHandler := registry.New(registry.Logger(log.New(ioutil.Discard, "", log.Lshortfile)))
	dockerRegistry = h.NewDockerRegistry(h.WithAuth(dockerConfigDir), h.WithSharedHandler(sharedRegistryHandler))

	dockerRegistry.SetInaccessible("cnbs/no-image-in-this-name")

	dockerRegistry.Start(t)
	defer dockerRegistry.Stop(t)

	os.Setenv("DOCKER_CONFIG", dockerRegistry.DockerDirectory)
	defer os.Unsetenv("DOCKER_CONFIG")

	spec.Run(t, "Index", testIndex, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testIndex(t *testing.T, when spec.G, it spec.S) {
	when("#NewIndex", func() {
		when("index name is invalid", func() {
			it("return error", func() {
				_, err := remote.NewIndex("-.bad-@!mage", authn.DefaultKeychain)
				h.AssertError(t, err, "could not parse reference: -.bad-@!mage")
			})
		})

		when("index name is valid", func() {
			it("create index with the specified name", func() {
				image := newTestIndexName()
				idxt, err := remote.NewIndex(image, authn.DefaultKeychain)
				h.AssertNil(t, err)
				h.AssertEq(t, image, idxt.Name())
			})
		})

		when("no options specified", func() {
			it("uses DockerManifestList as default mediatype", func() {
				idxt, _ := remote.NewIndexTest(newTestIndexName(), authn.DefaultKeychain)
				mediatype, _ := idxt.MediaType()
				h.AssertEq(t, mediatype, types.DockerManifestList)
			})
		})

		when("when index is found in registry", func() {
			it("use the index found in registry as base", func() {
			})

		})

		when("#WithIndexMediaTypes", func() {
			it("create index with the specified mediatype", func() {
				idxt, err := remote.NewIndexTest(
					newTestIndexName(),
					authn.DefaultKeychain,
					remote.WithIndexMediaTypes(imgutil.OCITypes))
				h.AssertNil(t, err)

				mediatype, err := idxt.MediaType()
				h.AssertNil(t, err)
				h.AssertEq(t, mediatype, types.OCIImageIndex)

			})
		})
	})

	when("#Add", func() {
		when("manifest is not in registry", func() {
			it("error (timeout) fetching manifest", func() {
				idx, err := remote.NewIndex("cnbs/test-index", authn.DefaultKeychain)
				h.AssertNil(t, err)

				manifestName := dockerRegistry.RepoName("cnbs/no-image-in-this-name")
				err = idx.Add(manifestName)
				h.AssertError(t, err, fmt.Sprintf("error fetching %s from registry", manifestName))
			})

		})

		when("manifest name is invalid", func() {
			it("error parsing reference", func() {
				idx, err := remote.NewIndex("some-bad-repo", authn.DefaultKeychain)
				h.AssertNil(t, err)

				manifestName := dockerRegistry.RepoName("cnbs/bad-@!mage")
				err = idx.Add(manifestName)
				h.AssertError(t, err, fmt.Sprintf("could not parse reference: %s", manifestName))
			})

		})

		when("manifest is in registry", func() {
			it("append manifest to index", func() {
				idx, err := remote.NewIndex("cnbs/test-index", authn.DefaultKeychain)
				h.AssertNil(t, err)

				manifestName := dockerRegistry.RepoName("cnbs/test-image:arm")
				img, err := remote.NewImage(
					manifestName,
					authn.DefaultKeychain,
					remote.WithDefaultPlatform(imgutil.Platform{
						Architecture: "arm",
						OS:           "linux",
					}),
				)
				h.AssertNil(t, img.Save())

				err = idx.Add(manifestName)
				h.AssertNil(t, err)
			})

		})

	})

	when("#Save", func() {
		when("manifest plaform fields are missing", func() {
			it("error storing in registry", func() {
				indexName := dockerRegistry.RepoName("cnbs/test-index-not-valid")
				idx, err := remote.NewIndex(indexName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				manifestName := dockerRegistry.RepoName("cnbs/test-image:arm")
				img, err := remote.NewImage(
					manifestName,
					authn.DefaultKeychain,
					remote.WithDefaultPlatform(imgutil.Platform{
						Architecture: "",
						OS:           "linux",
					}),
				)
				h.AssertNil(t, img.Save())

				h.AssertNil(t, idx.Add(manifestName))

				a := strings.Split(idx.Save().Error(), " ")

				h.AssertContains(t, a, "missing", "OS", "Architecture")
			})
		})

		when("index is valid to push", func() {
			it("store index in registry", func() {
				indexName := dockerRegistry.RepoName("cnbs/test-index-valid")
				idx, err := remote.NewIndex(indexName, authn.DefaultKeychain)
				h.AssertNil(t, err)

				manifestName := dockerRegistry.RepoName("cnbs/test-image:arm-linux")
				img, err := remote.NewImage(
					manifestName,
					authn.DefaultKeychain,
					remote.WithDefaultPlatform(imgutil.Platform{
						Architecture: "arm",
						OS:           "linux",
					}),
				)

				h.AssertNil(t, img.Save())

				h.AssertNil(t, idx.Add(manifestName))

				h.AssertNil(t, idx.Save())
			})
		})
	})
}
