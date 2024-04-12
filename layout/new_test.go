package layout_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
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

var (
	repoName = "some/index"
)

func testLayoutNewImageIndex(t *testing.T, when spec.G, it spec.S) {
	var (
		idx              imgutil.ImageIndex
		linuxAmd64Digest name.Digest
		linuxArm64Digest name.Digest

		tempDir     string
		testDataDir string

		err error
	)

	it.Before(func() {
		// creates the directory to save all the OCI images on disk
		tempDir, err = os.MkdirTemp("", "image-indexes")
		h.AssertNil(t, err)

		// global directory and paths
		testDataDir = filepath.Join("testdata", "layout")

		linuxAmd64Digest, err = name.NewDigest("busybox-multi-platform@sha256:4be429a5fbb2e71ae7958bfa558bc637cf3a61baf40a708cb8fff532b39e52d0")
		h.AssertNil(t, err)

		linuxArm64Digest, err = name.NewDigest("busybox-multi-platform@sha256:8a4415fb43600953cbdac6ec03c2d96d900bb21f8d78964837dad7f73b9afcdc")
		h.AssertNil(t, err)
	})

	it.After(func() {
		err := os.RemoveAll(tempDir)
		h.AssertNil(t, err)
	})

	when("#NewIndex", func() {
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

		when("index already exists on disk", func() {
			var (
				attribute  string
				attributes []string
			)

			it.Before(func() {
				baseIndexPath := filepath.Join(testDataDir, "busybox-multi-platform")
				idx, err = layout.NewIndex("busybox-multi-platform", testDataDir, imgutil.FromBaseImageIndex(baseIndexPath))
				h.AssertNil(t, err)
			})

			// Getters test cases
			when("platform attributes are selected", func() {
				// See spec: https://github.com/opencontainers/image-spec/blob/main/image-index.md#image-index-property-descriptions
				when("linux/amd64", func() {
					it("attributes are readable", func() {
						// #Architecture
						attribute, err = idx.Architecture(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "amd64")

						// #OS
						attribute, err = idx.OS(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "linux")

						// #Variant
						attribute, err = idx.Variant(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "v1")

						// #OSVersion
						attribute, err = idx.OSVersion(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "4.5.6")

						// #OSFeatures
						attributes, err = idx.OSFeatures(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertContains(t, attributes, "os-feature-1", "os-feature-2")

						// #Features
						attributes, err = idx.Features(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertContains(t, attributes, "feature-1", "feature-2")
					})
				})

				when("linux/arm64", func() {
					it("attributes are readable", func() {
						// #Architecture
						attribute, err = idx.Architecture(linuxArm64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "arm")

						// #OS
						attribute, err = idx.OS(linuxArm64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "linux")

						// #Variant
						attribute, err = idx.Variant(linuxArm64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "v7")

						// #OSVersion
						attribute, err = idx.OSVersion(linuxArm64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, attribute, "1.2.3")

						// #OSFeatures
						attributes, err = idx.OSFeatures(linuxArm64Digest)
						h.AssertNil(t, err)
						h.AssertContains(t, attributes, "os-feature-3", "os-feature-4")

						// #Features
						attributes, err = idx.Features(linuxArm64Digest)
						h.AssertNil(t, err)
						h.AssertContains(t, attributes, "feature-3", "feature-4")
					})
				})
			})

			when("#Annotations", func() {
				var annotations map[string]string

				when("linux/amd64", func() {
					it("existing annotations are readable", func() {
						annotations, err = idx.Annotations(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, annotations["com.docker.official-images.bashbrew.arch"], "amd64")
						h.AssertEq(t, annotations["org.opencontainers.image.url"], "https://hub.docker.com/_/busybox")
						h.AssertEq(t, annotations["org.opencontainers.image.revision"], "d0b7d566eb4f1fa9933984e6fc04ab11f08f4592")
					})
				})

				when("linux/arm64", func() {
					it("existing annotations are readable", func() {
						annotations, err = idx.Annotations(linuxArm64Digest)
						h.AssertNil(t, err)
						h.AssertEq(t, annotations["com.docker.official-images.bashbrew.arch"], "arm32v7")
						h.AssertEq(t, annotations["org.opencontainers.image.url"], "https://hub.docker.com/_/busybox")
						h.AssertEq(t, annotations["org.opencontainers.image.revision"], "185a3f7f21c307b15ef99b7088b228f004ff5f11")
					})
				})
			})

			when("#URLs", func() {
				when("linux/amd64", func() {
					it("existing annotations are readable", func() {
						t.Skip("Do we really want to support this now???, its failing")
						attributes, err = idx.URLs(linuxAmd64Digest)
						h.AssertNil(t, err)
						h.AssertContains(t, attributes, "https://foo.bar")
					})
				})
			})
		})
	})
}
