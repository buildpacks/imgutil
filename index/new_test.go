package index_test

import (
	"os"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/index"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestRemoteNew(t *testing.T) {
	spec.Run(t, "RemoteNew", testRemoteNew, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testRemoteNew(t *testing.T, when spec.G, it spec.S) {
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
		it("should have expected indexOptions", func() {
			idx, err = index.NewIndex("repo/name", index.WithInsecure(true), index.WithXDGRuntimePath(xdgPath))
			h.AssertNil(t, err)
			h.AssertEq(t, idx.(*imgutil.ManifestHandler).Options.InsecureRegistry, true)
		})
		it("should return an error when invalid repoName is passed", func() {
			idx, err = index.NewIndex("invalid/repoName", index.WithInsecure(true), index.WithXDGRuntimePath(xdgPath))
			h.AssertNotEq(t, err, nil)
		})
		it("should return ManifestHanler", func() {
			idx, err = index.NewIndex("repo/name", index.WithInsecure(true), index.WithXDGRuntimePath(xdgPath))
			h.AssertNil(t, err)

			_, ok := idx.(*imgutil.ManifestHandler)
			h.AssertEq(t, ok, true)
		})
	})
}
