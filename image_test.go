package imgutil

import (
	"fmt"
	h "github.com/buildpack/imgutil/testhelpers"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"math/rand"
	"testing"
	"time"
)

var (
	localTestRegistry *h.DockerRegistry
	dockerClient      *client.Client
)

type testCase struct {
	kind     string
	setUp    func() Image
	tearDown func(img Image)
}

func TestImage(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())

	localTestRegistry = h.NewDockerRegistry()
	localTestRegistry.Start(t)
	defer localTestRegistry.Stop(t)

	dockerClient = h.DockerCli(t)

	repoName := fmt.Sprintf("localhost:%s/test-image-%s", localTestRegistry.Port, h.RandString(10))

	setUpFake := func() Image {
		return NewFakeImage(t, repoName, "", "")
	}
	tearDownFake := func(img Image) {
		img.(*FakeImage).Cleanup()
	}

	setUpLocal := func() Image {
		h.CreateImageOnLocal(t, dockerClient, repoName, fmt.Sprintf(`
						FROM scratch
						LABEL repo_name_for_randomisation=%s
						LABEL mykey=myvalue other=data
					`, repoName), nil)
		img, err := NewLocalImage(repoName, dockerClient)
		h.AssertNil(t, err)
		return img
	}
	tearDownLocal := func(img Image) {
		h.AssertNil(t, h.DockerRmi(dockerClient, img.Name()))
	}

	setUpRemote := func() Image {
		h.CreateImageOnRemote(t, dockerClient, repoName, fmt.Sprintf(`
					FROM scratch
					LABEL repo_name_for_randomisation=%s
					LABEL mykey=myvalue other=data
				`, repoName), nil)

		img, err := NewRemoteImage(repoName, authn.DefaultKeychain)
		h.AssertNil(t, err)
		return img
	}
	tearDownRemote := func(img Image) {}

	testCases := []testCase{{"fake", setUpFake, tearDownFake}, {"local", setUpLocal, tearDownLocal}, {"remote", setUpRemote, tearDownRemote}}
	for _, tc := range testCases {
		testSetAndGetLabel(t, tc.kind, tc.setUp, tc.tearDown)
		testLabelMissing(t, tc.kind, tc.setUp, tc.tearDown)
	}
}

func testSetAndGetLabel(t *testing.T, kind string, setUp func() Image, tearDown func(img Image)) {
	t.Logf("test set and get label for type: %s", kind)
	img := setUp()
	defer tearDown(img)

	h.AssertNil(t, img.SetLabel("my-key", "my-value"))

	label, err := img.Label("my-key")
	h.AssertNil(t, err)
	h.AssertEq(t, label, "my-value")
}

func testLabelMissing(t *testing.T, kind string, setUp func() Image, tearDown func(img Image)) {
	t.Logf("test missing label for type: %s", kind)
	img := setUp()
	defer tearDown(img)

	label, err := img.Label("missing-label")
	h.AssertNil(t, err)
	h.AssertEq(t, label, "")
}
