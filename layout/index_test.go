package layout_test

import (
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

func TestLayoutIndex(t *testing.T) {
	spec.Run(t, "LayoutNewIndex", testNewIndex, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testNewIndex(t *testing.T, when spec.G, it spec.S) {
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
		_ = idx
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
					imgutil.WithXDGRuntimePath(tempDir),
				)
				h.AssertNil(t, err)
			})

			it("ignores FromBaseIndex if it doesn't exist", func() {
				idx, err = layout.NewIndex(
					repoName,
					imgutil.WithXDGRuntimePath(tempDir),
					imgutil.FromBaseIndex("non-existent/index"),
				)
				h.AssertNil(t, err)
			})

			it("creates empty image index with Docker media-types", func() {
				idx, err = layout.NewIndex(
					repoName,
					imgutil.WithXDGRuntimePath(tempDir),
					imgutil.WithMediaType(types.DockerManifestList),
				)
				h.AssertNil(t, err)
			})
		})
	})
}
