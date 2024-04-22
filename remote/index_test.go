package remote_test

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/remote"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestRemoteNewIndex(t *testing.T) {
	spec.Run(t, "RemoteNewIndex", testNewIndex, spec.Parallel(), spec.Report(report.Terminal{}))
}

// FIXME: these tests should push and pull from a local registry to avoid getting rate limited
func testNewIndex(t *testing.T, when spec.G, it spec.S) {
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
			idx, err = remote.NewIndex(
				"busybox:1.36-musl",
				imgutil.WithInsecure(),
				imgutil.WithKeychain(authn.DefaultKeychain),
				imgutil.WithXDGRuntimePath(xdgPath),
			)
			h.AssertNil(t, err)

			imgIx, ok := idx.(*imgutil.CNBIndex)
			h.AssertEq(t, ok, true)
			h.AssertEq(t, imgIx.XdgPath, xdgPath)
			h.AssertEq(t, imgIx.RepoName, "busybox:1.36-musl")
		})

		it("should return an error when index with the given repoName doesn't exists", func() {
			_, err = remote.NewIndex(
				"my-index",
				imgutil.WithKeychain(authn.DefaultKeychain),
				imgutil.FromBaseIndex("some-not-exist-index"),
			)
			h.AssertNotEq(t, err, nil)
		})

		it("should return ImageIndex with expected output", func() {
			idx, err = remote.NewIndex(
				"my-index",
				imgutil.WithKeychain(authn.DefaultKeychain),
				imgutil.FromBaseIndex("busybox:1.36-musl"),
			)
			h.AssertNil(t, err)

			imgIx, ok := idx.(*imgutil.CNBIndex)
			h.AssertEq(t, ok, true)

			mfest, err := imgIx.IndexManifest()
			h.AssertNil(t, err)
			h.AssertNotEq(t, mfest, nil)
			h.AssertEq(t, len(mfest.Manifests), 8)
		})

		it("should able to call #ImageIndex", func() {
			idx, err = remote.NewIndex(
				"my-index",
				imgutil.WithKeychain(authn.DefaultKeychain),
				imgutil.FromBaseIndex("busybox:1.36-musl"),
			)
			h.AssertNil(t, err)

			imgIx, ok := idx.(*imgutil.CNBIndex)
			h.AssertEq(t, ok, true)

			// linux/amd64
			hash1, err := v1.NewHash(
				"sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
			)
			h.AssertNil(t, err)

			_, err = imgIx.ImageIndex.ImageIndex(hash1)
			h.AssertNotEq(t, err.Error(), "empty index")
		})

		it("should able to call #Image", func() {
			idx, err = remote.NewIndex(
				"my-index",
				imgutil.WithKeychain(authn.DefaultKeychain),
				imgutil.FromBaseIndex("busybox:1.36-musl"),
			)
			h.AssertNil(t, err)

			imgIdx, ok := idx.(*imgutil.CNBIndex)
			h.AssertEq(t, ok, true)

			// linux/amd64
			hash1, err := v1.NewHash(
				"sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
			)
			h.AssertNil(t, err)

			_, err = imgIdx.Image(hash1)
			h.AssertNil(t, err)
		})
	})
}
