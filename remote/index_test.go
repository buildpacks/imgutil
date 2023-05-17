package remote_test

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

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

}
