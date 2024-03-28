package imgutil_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/fakes"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestUtils(t *testing.T) {
	spec.Run(t, "Utils", testUtils, spec.Sequential(), spec.Report(report.Terminal{}))
}

type FakeIndentifier struct {
	hash string
}

func NewFakeIdentifier(hash string) FakeIndentifier {
	return FakeIndentifier{
		hash: hash,
	}
}

func (f FakeIndentifier) String() string {
	return f.hash
}

func testUtils(t *testing.T, when spec.G, it spec.S) {
	const fakeHash = "sha256:13553267bf712ee37527bdbbde41115b287062b72e2d54c573edf68d88e3cb4f"
	when("#MutateManifest", func() {
		var (
			img *fakes.Image
		)
		it.Before(func() {
			img = fakes.NewImage("some-name", fakeHash, NewFakeIdentifier(fakeHash))
		})
		it("should muatet Image", func() {
			var (
				annotations = map[string]string{"some-key": "some-value"}
				urls        = []string{"some-url1", "some-url2"}
				os          = "some-os"
				arch        = "some-arch"
				variant     = "some-variant"
				osVersion   = "some-os-version"
				features    = []string{"some-feat1", "some-feat2"}
				osFeatures  = []string{"some-os-feat1", "some-os-feat2"}
			)

			exptConfig, err := img.ConfigFile()
			h.AssertNil(t, err)
			h.AssertNotNil(t, exptConfig)

			img, err := imgutil.MutateManifest(img, func(c *v1.Manifest) {
				c.Annotations = annotations
				c.Config.URLs = urls
				c.Config.Platform.OS = os
				c.Config.Platform.Architecture = arch
				c.Config.Platform.Variant = variant
				c.Config.Platform.OSVersion = osVersion
				c.Config.Platform.Features = features
				c.Config.Platform.OSFeatures = osFeatures
			})

			h.AssertNil(t, err)
			mfest, err := img.Manifest()
			h.AssertNil(t, err)
			h.AssertNotNil(t, mfest)

			h.AssertEq(t, mfest.Annotations, annotations)
			h.AssertEq(t, mfest.Subject.URLs, urls)
			h.AssertEq(t, mfest.Subject.Platform.OS, os)
			h.AssertEq(t, mfest.Subject.Platform.Architecture, arch)
			h.AssertEq(t, mfest.Subject.Platform.Variant, variant)
			h.AssertEq(t, mfest.Subject.Platform.OSVersion, osVersion)
			h.AssertEq(t, mfest.Subject.Platform.Features, features)
			h.AssertEq(t, mfest.Subject.Platform.OSFeatures, osFeatures)

			orgConfig, err := img.ConfigFile()
			h.AssertNil(t, err)
			h.AssertNotNil(t, orgConfig)

			h.AssertEq(t, orgConfig, exptConfig)
		})
	})
}
