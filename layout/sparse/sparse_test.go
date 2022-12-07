package sparse_test

import (
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil/layout/sparse"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestImage(t *testing.T) {
	spec.Run(t, "LayoutSparseImage", testImage, spec.Report(report.Terminal{}))
}

func testImage(t *testing.T, when spec.G, it spec.S) {
	var (
		testImage v1.Image
		tmpDir    string
		imagePath string
		err       error
	)

	it.Before(func() {
		testImage = h.RemoteRunnableBaseImage(t)

		// creates the directory to save all the OCI images on disk
		tmpDir, err = os.MkdirTemp("", "layout-sparse")
		h.AssertNil(t, err)
	})

	it.After(func() {
		// removes all images created
		os.RemoveAll(tmpDir)
	})

	when("#Save", func() {
		it.Before(func() {
			imagePath = filepath.Join(tmpDir, "sparse-layout-image")
		})

		it.After(func() {
			// removes all images created
			os.RemoveAll(imagePath)
		})

		when("name(s) provided", func() {
			it("creates an image without layers and org.opencontainers.image.ref.name annotation", func() {
				image, err := sparse.NewImage(imagePath, testImage)
				h.AssertNil(t, err)

				// save on disk in OCI
				err = image.Save("my-additional-tag")
				h.AssertNil(t, err)

				//  expected blobs: manifest, config, layer
				h.AssertBlobsLen(t, imagePath, 2)

				// assert additional name
				index := h.ReadIndexManifest(t, imagePath)
				h.AssertEq(t, len(index.Manifests), 1)
				h.AssertEq(t, "my-additional-tag", index.Manifests[0].Annotations["org.opencontainers.image.ref.name"])
			})

			it("failed on saved when more than one name is provided", func() {
				image, err := sparse.NewImage(imagePath, testImage)
				h.AssertNil(t, err)

				// save on disk in OCI
				err = image.Save("name1", "name2")
				h.AssertError(t, err, "multiple additional names [name1 name2] are not allow when OCI layout is used")
			})
		})

		when("no additional names are provided", func() {
			it("creates an image without layers", func() {
				image, err := sparse.NewImage(imagePath, testImage)
				h.AssertNil(t, err)

				// save
				err = image.Save()
				h.AssertNil(t, err)

				// expected blobs: manifest, config
				h.AssertBlobsLen(t, imagePath, 2)
			})
		})
	})
}
