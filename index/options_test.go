package index_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
	// h "github.com/buildpacks/imgutil/testhelpers"
)

func TestRemoteOptions(t *testing.T) {
	spec.Run(t, "RemoteNew", testRemoteOptions, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testRemoteOptions(t *testing.T, when spec.G, it spec.S) {
	when("#NewIndex", func() {
		it("should have expected indexOptions", func() {})
		it("should return an error when invalid repoName is passed", func() {})
		it("should return an error when index with the given repoName doesn't exists", func() {})
		it("should return ImageIndex with expected output", func() {})
		it("should able to call #ImageIndex", func() {})
		it("should able to call #Image", func() {})
	})
}
