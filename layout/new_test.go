package layout_test

import (
	"os"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/index"
	"github.com/buildpacks/imgutil/layout"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestLayoutNewImageIndex(t *testing.T) {
	spec.Run(t, "LayoutNewImageIndex", testLayoutNewImageIndex, spec.Parallel(), spec.Report(report.Terminal{}))
}

var (
	repoName = "some/index"
)

func testLayoutNewImageIndex(t *testing.T, when spec.G, it spec.S) {
	var (
		idx     imgutil.ImageIndex
		xdgPath string
		err     error
	)

	it.Before(func() {
		// creates the directory to save all the OCI images on disk
		xdgPath, err = os.MkdirTemp("", "image-indexes")
		h.AssertNil(t, err)
	})

	it.After(func() {
		err := os.RemoveAll(xdgPath)
		h.AssertNil(t, err)
	})

	when("#NewIndex", func() {
		it.Before(func() {
			idx, err = index.NewIndex(
				repoName,
				index.WithFormat(types.OCIImageIndex),
				index.WithXDGRuntimePath(xdgPath),
			)
			h.AssertNil(t, err)
		})
		it("should have expected indexOptions", func() {
			idx, err = layout.NewIndex(
				repoName,
				xdgPath,
			)
			h.AssertNil(t, err)

			imgIdx, ok := idx.(*layout.ImageIndex)
			h.AssertEq(t, ok, true)
			h.AssertEq(t, imgIdx.RepoName, repoName)
			h.AssertEq(t, imgIdx.XdgPath, xdgPath)

			err = idx.Delete()
			h.AssertNil(t, err)
		})
		it("should return an error when invalid repoName is passed", func() {
			idx, err = layout.NewIndex(
				repoName+"Image",
				xdgPath,
			)
			h.AssertNotNil(t, err)
		})
		it("should return ImageIndex with expected output", func() {
			idx, err = layout.NewIndex(
				repoName,
				xdgPath,
			)
			h.AssertNil(t, err)
			h.AssertNotNil(t, idx)

			err = idx.Delete()
			h.AssertNil(t, err)
		})
		it("should able to call #ImageIndex", func() {
			idx, err = layout.NewIndex(
				repoName,
				xdgPath,
			)
			h.AssertNil(t, err)

			imgIdx, ok := idx.(*layout.ImageIndex)
			h.AssertEq(t, ok, true)

			hash, err := v1.NewHash("sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a")
			h.AssertNil(t, err)

			_, err = imgIdx.ImageIndex.ImageIndex(hash)
			h.AssertNotEq(t, err.Error(), "empty index")

			err = idx.Delete()
			h.AssertNil(t, err)
		})
		it("should able to call #Image", func() {
			idx, err = layout.NewIndex(
				repoName,
				xdgPath,
			)
			h.AssertNil(t, err)

			imgIdx, ok := idx.(*layout.ImageIndex)
			h.AssertEq(t, ok, true)

			hash, err := v1.NewHash("sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a")
			h.AssertNil(t, err)

			_, err = imgIdx.ImageIndex.Image(hash)
			h.AssertNotEq(t, err.Error(), "empty index")

			err = idx.Delete()
			h.AssertNil(t, err)
		})
	})
}
