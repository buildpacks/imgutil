package layout_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/layout"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestLayoutNewImageIndex(t *testing.T) {
	spec.Run(t, "LayoutNewImageIndex", testLayoutNewImageIndex, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testLayoutNewImageIndex(t *testing.T, when spec.G, it spec.S) {
	var (
		idx      imgutil.ImageIndex
		tempDir  string
		repoName string
		err      error
	)

	it.Before(func() {
		// creates the directory to save all the OCI images on disk
		tempDir, err = os.MkdirTemp("", "image-indexes")
		h.AssertNil(t, err)

		// global directory and paths
		testDataDir = filepath.Join("testdata", "layout")
	})

	it.After(func() {
		err := os.RemoveAll(tempDir)
		h.AssertNil(t, err)
	})

	when("#NewIndex", func() {
		it.Before(func() {
			repoName = "some/index"
		})

		when("index doesn't exists on disk", func() {
			it("creates empty image index", func() {
				idx, err = layout.NewIndex(
					repoName,
					tempDir,
				)
				h.AssertNil(t, err)
			})

			it("ignores FromBaseImageIndex if it doesn't exist", func() {
				idx, err = layout.NewIndex(
					repoName,
					tempDir,
					imgutil.FromBaseImageIndex("non-existent/index"),
				)
				h.AssertNil(t, err)
			})

			it("creates empty image index with Docker media-types", func() {
				idx, err = layout.NewIndex(
					repoName,
					tempDir,
					imgutil.WithFormat(types.DockerManifestList),
				)
				h.AssertNil(t, err)
				imgIdx, ok := idx.(*layout.ImageIndex)
				h.AssertEq(t, ok, true)
				h.AssertEq(t, imgIdx.Format, types.DockerManifestList)
			})

			it("should return an error when invalid repoName is passed", func() {
				failingName := repoName + ":🧨"
				idx, err = layout.NewIndex(
					failingName,
					tempDir,
				)
				h.AssertNotNil(t, err)
				h.AssertError(t, err, fmt.Sprintf("could not parse reference: %s", failingName))

				// when insecure
				idx, err = layout.NewIndex(
					failingName,
					tempDir,
					imgutil.PullInsecure(),
				)
				h.AssertNotNil(t, err)
				h.AssertError(t, err, fmt.Sprintf("could not parse reference: %s", failingName))
			})

			it("error when an image index already exists at path and it is not reuse", func() {
				localPath := filepath.Join(testDataDir, "busybox-multi-platform")
				idx, err = layout.NewIndex("busybox-multi-platform", testDataDir)
				h.AssertNotNil(t, err)
				h.AssertError(t, err, fmt.Sprintf("an image index already exists at %s use FromBaseImageIndex or FromBaseImageIndexInstance options to create a new instance", localPath))
			})
		})
	})
}
