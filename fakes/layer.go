package fakes

import (
	"archive/tar"
	"bytes"
	"crypto"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Layer returns a layer with pseudo-randomly generated content.
func Layer(byteSize int64, mt types.MediaType, options ...Option) (v1.Layer, error) {
	o := getOptions(options)
	rng := rand.New(o.source) //nolint:gosec

	fileName := fmt.Sprintf("random_file_%d.txt", rng.Int())

	// Hash the contents as we write it out to the buffer.
	var b bytes.Buffer
	hasher := crypto.SHA256.New()
	mw := io.MultiWriter(&b, hasher)

	// Write a single file with a random name and random contents.
	tw := tar.NewWriter(mw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     fileName,
		Size:     byteSize,
		Typeflag: tar.TypeReg,
	}); err != nil {
		return nil, err
	}
	if _, err := io.CopyN(tw, rng, byteSize); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	h := v1.Hash{
		Algorithm: "sha256",
		Hex:       hex.EncodeToString(hasher.Sum(make([]byte, 0, hasher.Size()))),
	}

	if o.withHash != (v1.Hash{}) {
		h = o.withHash
	}

	return partial.UncompressedToLayer(&uncompressedLayer{
		diffID:    h,
		mediaType: mt,
		content:   b.Bytes(),
	})
}
