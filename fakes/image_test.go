package fakes_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpack/imgutil"
	"github.com/buildpack/imgutil/fakes"
	h "github.com/buildpack/imgutil/testhelpers"
)

var localTestRegistry *h.DockerRegistry

func newRepoName() string {
	return "test-image-" + h.RandString(10)
}

func TestFake(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())

	localTestRegistry = h.NewDockerRegistry()
	localTestRegistry.Start(t)
	defer localTestRegistry.Stop(t)

	spec.Run(t, "FakeImage", testFake, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testFake(t *testing.T, when spec.G, it spec.S) {
	when("#SavedNames", func() {
		when("additional names are provided during save", func() {
			var (
				repoName        = newRepoName()
				additionalNames = []string{
					newRepoName(),
					newRepoName(),
				}
			)

			it("returns list of saved names", func() {
				image := fakes.NewImage(repoName, "", "")

				_ = image.Save(additionalNames...)

				names := image.SavedNames()
				h.AssertContains(t, names, append(additionalNames, repoName)...)
			})

			when("an image name is not valid", func() {
				it("returns a list of image names with errors", func() {
					badImageName := repoName + ":🧨"

					image := fakes.NewImage(repoName, "", "")

					err := image.Save(append([]string{badImageName}, additionalNames...)...)
					saveErr, ok := err.(imgutil.SaveError)
					h.AssertEq(t, ok, true)
					h.AssertEq(t, len(saveErr.Errors), 1)
					h.AssertEq(t, saveErr.Errors[0].ImageName, badImageName)
					h.AssertError(t, saveErr.Errors[0].Cause, "could not parse reference")

					names := image.SavedNames()
					h.AssertContains(t, names, append(additionalNames, repoName)...)
					h.AssertDoesNotContain(t, names, badImageName)
				})
			})
		})
	})
}
