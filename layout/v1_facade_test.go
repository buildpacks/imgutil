package layout

import (
	"errors"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestV1Facade(t *testing.T) {
	spec.Run(t, "V1Facade", testV1Facade, spec.Parallel(), spec.Report(report.Terminal{}))
}

// dataLessLayer models a layer from a sparse image: its content isn't on disk,
// so Compressed returns an error and hasData reports false.
type dataLessLayer struct {
	v1.Layer
}

func (l dataLessLayer) Compressed() (io.ReadCloser, error) {
	return nil, errors.New("no data")
}

func testV1Facade(t *testing.T, when spec.G, it spec.S) {
	when("#newLayerOrFacadeFrom", func() {
		when("the layer index is out of bounds", func() {
			it("errors when the config has fewer diffIDs than the manifest has layers", func() {
				configFile := v1.ConfigFile{RootFS: v1.RootFS{DiffIDs: []v1.Hash{
					{Algorithm: "sha256", Hex: "0000000000000000000000000000000000000000000000000000000000000000"},
				}}}
				manifestFile := v1.Manifest{Layers: []v1.Descriptor{
					{Digest: v1.Hash{Algorithm: "sha256", Hex: "1111111111111111111111111111111111111111111111111111111111111111"}},
					{Digest: v1.Hash{Algorithm: "sha256", Hex: "2222222222222222222222222222222222222222222222222222222222222222"}},
				}}

				_, err := newLayerOrFacadeFrom(configFile, manifestFile, 1, dataLessLayer{})

				h.AssertError(t, err, "failed to find layer for index 1 in config file")
			})

			it("errors when the manifest has fewer layers than the config has diffIDs", func() {
				configFile := v1.ConfigFile{RootFS: v1.RootFS{DiffIDs: []v1.Hash{
					{Algorithm: "sha256", Hex: "0000000000000000000000000000000000000000000000000000000000000000"},
					{Algorithm: "sha256", Hex: "3333333333333333333333333333333333333333333333333333333333333333"},
				}}}
				manifestFile := v1.Manifest{Layers: []v1.Descriptor{
					{Digest: v1.Hash{Algorithm: "sha256", Hex: "1111111111111111111111111111111111111111111111111111111111111111"}},
				}}

				_, err := newLayerOrFacadeFrom(configFile, manifestFile, 1, dataLessLayer{})

				h.AssertError(t, err, "failed to find layer for index 1 in manifest file")
			})
		})

		when("the layer index is in bounds", func() {
			it("returns a facade with the diffID, digest and size from the image", func() {
				diffID := v1.Hash{Algorithm: "sha256", Hex: "0000000000000000000000000000000000000000000000000000000000000000"}
				digest := v1.Hash{Algorithm: "sha256", Hex: "1111111111111111111111111111111111111111111111111111111111111111"}
				configFile := v1.ConfigFile{RootFS: v1.RootFS{DiffIDs: []v1.Hash{diffID}}}
				manifestFile := v1.Manifest{Layers: []v1.Descriptor{{Digest: digest, Size: 42}}}

				layer, err := newLayerOrFacadeFrom(configFile, manifestFile, 0, dataLessLayer{})
				h.AssertNil(t, err)

				gotDiffID, err := layer.DiffID()
				h.AssertNil(t, err)
				h.AssertEq(t, gotDiffID, diffID)

				gotDigest, err := layer.Digest()
				h.AssertNil(t, err)
				h.AssertEq(t, gotDigest, digest)

				gotSize, err := layer.Size()
				h.AssertNil(t, err)
				h.AssertEq(t, gotSize, int64(42))
			})
		})
	})
}
