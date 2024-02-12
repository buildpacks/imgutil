package index_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil/index"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestRemoteOptions(t *testing.T) {
	spec.Run(t, "RemoteNew", testRemoteOptions, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testRemoteOptions(t *testing.T, when spec.G, it spec.S) {
	var (
		ops  = &index.Options{}
		opts = []index.Option(nil)
	)
	when("#NewIndex", func() {
		it.Before(func() {
			ops = &index.Options{}
			opts = []index.Option(nil)
		})
		it("should have expected xdgpath value", func() {
			opts = append(opts, index.WithXDGRuntimePath("xdgPath"))
			for _, op := range opts {
				op(ops)
			}

			h.AssertEq(t, ops.XDGRuntimePath(), "xdgPath")
		})
		it("should return an error when invalid repoName is passed", func() {
			opts = append(opts, index.WithRepoName("repo/name"))
			for _, op := range opts {
				h.AssertNil(t, op(ops))
			}

			h.AssertEq(t, ops.RepoName(), "repo/name")
		})
		it("should return an error when index with the given repoName doesn't exists", func() {
			opts = append(opts, index.WithRepoName("repoName"))
			for _, op := range opts {
				err := op(ops)
				h.AssertNotEq(t, err, nil)
			}

			h.AssertEq(t, ops.RepoName(), "")
		})
		it("should have expected insecure value", func() {
			opts = append(opts, index.WithInsecure(true))
			for _, op := range opts {
				op(ops)
			}

			h.AssertEq(t, ops.Insecure(), true)
		})
		it("should have expected format value", func() {
			opts = append(opts, index.WithFormat(types.DockerManifestList))
			for _, op := range opts {
				op(ops)
			}

			h.AssertEq(t, ops.Format(), types.DockerManifestList)
		})
		it("should have expected manifestOnly", func() {
			opts = append(opts, index.WithManifestOnly(true))
			for _, op := range opts {
				op(ops)
			}

			h.AssertEq(t, ops.ManifestOnly(), true)
		})
	})
}
