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
	spec.Run(t, "RemoteNew", testRemoteNew, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testRemoteNew(t *testing.T, when spec.G, it spec.S) {
	when("#NewIndex", func() {
		var (
			idx     imgutil.ImageIndex
			err     error
			xdgPath = "xdgPath"
		)
		it.After(func() {
			h.AssertNil(t, os.RemoveAll(xdgPath))
		})
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
