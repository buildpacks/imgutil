package testhelpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil/archive"
)

var gzipMagicHeader = []byte{'\x1f', '\x8b'}

type TarEntryAssertion func(t *testing.T, header *tar.Header, data []byte)

func AssertOnTarEntry(t *testing.T, tarPath, entryPath string, assertFns ...TarEntryAssertion) {
	t.Helper()

	tarFile, err := os.Open(tarPath)
	AssertNil(t, err)
	defer tarFile.Close()

	header, data, err := readTarFileEntry(tarFile, entryPath)
	AssertNil(t, err)

	for _, fn := range assertFns {
		fn(t, header, data)
	}
}

func readTarFileEntry(reader io.Reader, entryPath string) (*tar.Header, []byte, error) {
	var (
		gzipReader *gzip.Reader
		err        error
	)

	headerBytes, isGzipped, err := isGzipped(reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "checking if reader")
	}
	reader = io.MultiReader(bytes.NewReader(headerBytes), reader)

	if isGzipped {
		gzipReader, err = gzip.NewReader(reader)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to create gzip reader")
		}
		reader = gzipReader
		defer gzipReader.Close()
	}

	return archive.ReadTarEntry(reader, entryPath)
}

func isGzipped(reader io.Reader) (headerBytes []byte, isGzipped bool, err error) {
	magicHeader := make([]byte, 2)
	n, err := reader.Read(magicHeader)
	if n == 0 && err == io.EOF {
		return magicHeader, false, nil
	}
	if err != nil {
		return magicHeader, false, err
	}
	return magicHeader, bytes.Equal(magicHeader, gzipMagicHeader), nil
}

func HasModTime(expectedTime time.Time) TarEntryAssertion {
	return func(t *testing.T, header *tar.Header, _ []byte) {
		t.Helper()
		if header.ModTime.UnixNano() != expectedTime.UnixNano() {
			t.Fatalf("expected '%s' to have mod time '%s', but got '%s'", header.Name, expectedTime, header.ModTime)
		}
	}
}

func DoesNotHaveModTime(expectedTime time.Time) TarEntryAssertion {
	return func(t *testing.T, header *tar.Header, _ []byte) {
		t.Helper()
		if header.ModTime.UnixNano() == expectedTime.UnixNano() {
			t.Fatalf("expected '%s' to not have mod time '%s'", header.Name, expectedTime)
		}
	}
}
