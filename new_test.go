package imgutil_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestNewIndex(t *testing.T) {
	spec.Run(t, "IndexOptions", testNewIndex, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testNewIndex(t *testing.T, when spec.G, it spec.S) {
	when("#NewManifestHandler", func() {
		it("should create with expected Index", func() {
			ih := imgutil.NewManifestHandler(empty.Index, imgutil.IndexOptions{})
			h.AssertEq(t, ih.ImageIndex, empty.Index)
		})
		it("should create with expected Options", func() {
			ops := imgutil.IndexOptions{
				XdgPath:          "xdgPath",
				Reponame:         "some/repo",
				InsecureRegistry: false,
			}

			ih := imgutil.NewManifestHandler(empty.Index, ops)
			h.AssertEq(t, ih.Options.InsecureRegistry, ops.InsecureRegistry)
			h.AssertEq(t, ih.Options.Reponame, ops.Reponame)
			h.AssertEq(t, ih.Options.XdgPath, ops.XdgPath)
			h.AssertEq(t, ih.Options.KeyChain, ops.KeyChain)
		})
		it("should create ManifestHandlers with not Nil maps and slices", func() {
			ih := imgutil.NewManifestHandler(empty.Index, imgutil.IndexOptions{})
			h.AssertEq(t, len(ih.Annotate.Instance), 0)
			h.AssertEq(t, len(ih.RemovedManifests), 0)
			h.AssertEq(t, len(ih.Images), 0)
		})
	})
	when("#NewEmptyDockerIndex", func() {
		it("should return an empty docker index", func() {
			idx := imgutil.NewEmptyDockerIndex()
			h.AssertNotNil(t, idx)

			digest, err := idx.Digest()
			h.AssertNil(t, err)
			h.AssertNotEq(t, digest, v1.Hash{})

			format, err := idx.MediaType()
			h.AssertNil(t, err)
			h.AssertEq(t, format, types.DockerManifestList)
		})
	})
	when("#NewStringSet", func() {
		it("should return not nil StringSet instance", func() {
			stringSet := imgutil.NewStringSet()
			h.AssertNotNil(t, stringSet)
			h.AssertEq(t, stringSet.StringSlice(), []string(nil))
		})
	})
}
