package layout

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrlayout "github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestWrite(t *testing.T) {
	spec.Run(t, "Write", testWrite, spec.Parallel(), spec.Report(report.Terminal{}))
}

type closeTrackingReader struct {
	io.Reader
	closed bool
}

func (r *closeTrackingReader) Close() error {
	r.closed = true
	return nil
}

func testWrite(t *testing.T, when spec.G, it spec.S) {
	when("#writeBlob", func() {
		when("the blob already exists", func() {
			it("closes the reader", func() {
				layoutPath := Path{Path: ggcrlayout.Path(t.TempDir())}
				hash := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}
				blobDir := filepath.Join(string(layoutPath.Path), "blobs", hash.Algorithm)
				h.AssertNil(t, os.MkdirAll(blobDir, os.ModePerm))
				h.AssertNil(t, os.WriteFile(filepath.Join(blobDir, hash.Hex), []byte("data"), 0600))

				rc := &closeTrackingReader{Reader: strings.NewReader("data")}
				h.AssertNil(t, layoutPath.writeBlob(hash, 4, rc, nil))
				h.AssertEq(t, rc.closed, true)
			})
		})
	})
}
