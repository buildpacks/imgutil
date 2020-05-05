package layer_test

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/buildpacks/imgutil/layer"
	h "github.com/buildpacks/imgutil/testhelpers"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestWindowsWriter(t *testing.T) {
	spec.Run(t, "windows-writer", testWindowsWriter, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testWindowsWriter(t *testing.T, when spec.G, it spec.S) {
	when("#WriteHeader", func() {
		it("writes required entries", func() {
			var err error

			f, err := ioutil.TempFile("", "windows-writer.tar")
			h.AssertNil(t, err)
			defer func() { f.Close(); os.Remove(f.Name()) }()

			lw := layer.NewWindowsWriter(f)

			h.AssertNil(t, lw.WriteHeader(&tar.Header{
				Name:     "/cnb/my-file",
				Typeflag: tar.TypeReg,
			}))

			h.AssertNil(t, lw.Close())

			f.Seek(0, 0)

			tr := tar.NewReader(f)

			th, _ := tr.Next()
			h.AssertEq(t, th.Name, "Files")
			h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

			th, _ = tr.Next()
			h.AssertEq(t, th.Name, "Hives")
			h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

			th, _ = tr.Next()
			h.AssertEq(t, th.Name, "Files/cnb")
			h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

			th, _ = tr.Next()
			h.AssertEq(t, th.Name, "Files/cnb/my-file")
			h.AssertEq(t, th.Typeflag, byte(tar.TypeReg))

			_, err = tr.Next()
			h.AssertError(t, err, "EOF")
		})

		when("duplicate parent directories", func() {
			it("only writes parents once", func() {
				var err error

				f, err := ioutil.TempFile("", "windows-writer.tar")
				h.AssertNil(t, err)
				defer func() { f.Close(); os.Remove(f.Name()) }()

				lw := layer.NewWindowsWriter(f)

				h.AssertNil(t, lw.WriteHeader(&tar.Header{
					Name:     "/cnb/lifecycle/first-file",
					Typeflag: tar.TypeReg,
				}))

				h.AssertNil(t, lw.WriteHeader(&tar.Header{
					Name:     "/cnb/sibling-dir",
					Typeflag: tar.TypeDir,
				}))

				h.AssertNil(t, lw.Close())

				f.Seek(0, 0)
				tr := tar.NewReader(f)

				th, _ := tr.Next()
				h.AssertEq(t, th.Name, "Files")
				h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

				th, _ = tr.Next()
				h.AssertEq(t, th.Name, "Hives")
				h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

				th, _ = tr.Next()
				h.AssertEq(t, th.Name, "Files/cnb")
				h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

				th, _ = tr.Next()
				h.AssertEq(t, th.Name, "Files/cnb/lifecycle")
				h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

				th, _ = tr.Next()
				h.AssertEq(t, th.Name, "Files/cnb/lifecycle/first-file")
				h.AssertEq(t, th.Typeflag, byte(tar.TypeReg))

				th, _ = tr.Next()
				h.AssertEq(t, th.Name, "Files/cnb/sibling-dir")
				h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

				_, err = tr.Next()
				h.AssertError(t, err, "EOF")
			})
		})

		when("header.Name is invalid", func() {
			it("returns an error", func() {
				lw := layer.NewWindowsWriter(&bytes.Buffer{})

				h.AssertError(t, lw.WriteHeader(&tar.Header{
					Name:     `c:\windows-path.txt`,
					Typeflag: tar.TypeReg,
				}), "invalid header name: must be absolute, posix path")

				h.AssertError(t, lw.WriteHeader(&tar.Header{
					Name:     `\lonelyfile`,
					Typeflag: tar.TypeDir,
				}), "invalid header name: must be absolute, posix path")

				h.AssertError(t, lw.WriteHeader(&tar.Header{
					Name:     "Files/cnb/lifecycle/first-file",
					Typeflag: tar.TypeDir,
				}), "invalid header name: must be absolute, posix path")
			})
		})
	})

	when("#Close", func() {
		it("writes required parent dirs on empty layer", func() {
			var err error

			f, err := ioutil.TempFile("", "windows-writer.tar")
			h.AssertNil(t, err)
			defer func() { f.Close(); os.Remove(f.Name()) }()

			lw := layer.NewWindowsWriter(f)

			err = lw.Close()
			h.AssertNil(t, err)

			f.Seek(0, 0)
			tr := tar.NewReader(f)

			th, _ := tr.Next()
			h.AssertEq(t, th.Name, "Files")
			h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

			th, _ = tr.Next()
			h.AssertEq(t, th.Name, "Hives")
			h.AssertEq(t, th.Typeflag, byte(tar.TypeDir))

			_, err = tr.Next()
			h.AssertError(t, err, "EOF")
		})
	})
}
