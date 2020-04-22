package layer_test

import (
	"archive/tar"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil/fakes"
	"github.com/buildpacks/imgutil/layer"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestTarWriterFactory(t *testing.T) {
	spec.Run(t, "tar-writer-factory", testTarWriterFactory, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testTarWriterFactory(t *testing.T, when spec.G, it spec.S) {
	when("#NewTarWriter", func() {
		it("returns a regular tar writer for posix-based images", func() {
			image := fakes.NewImage("fake-image", "", nil)
			image.SetPlatform("linux", "", "")
			factory, err := layer.NewTarWriterFactory(image)
			h.AssertNil(t, err)

			_, ok := factory.NewTarWriter(nil).(*tar.Writer)
			if !ok {
				t.Fatal("returned tar writer was not a regular tar writer")
			}
		})

		it("returns a Windows tar writer for Windows-based images", func() {
			image := fakes.NewImage("fake-image", "", nil)
			image.SetPlatform("windows", "", "")
			factory, err := layer.NewTarWriterFactory(image)
			h.AssertNil(t, err)

			_, ok := factory.NewTarWriter(nil).(*layer.WindowsWriter)
			if !ok {
				t.Fatal("returned tar writer was not a Windows tar writer")
			}
		})
	})
}
