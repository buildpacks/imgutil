package locallayout

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// addDirToTar walks a directory and writes entries describing dir and all of its children files to the provided *tar.Writer
func addDirToTar(tw *tar.Writer, dir string) error {
	dir = filepath.Clean(dir)

	return filepath.Walk(dir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return addFileToTar(tw, file, dir, fi)
	})
}

// addFileToTar writes an entry describing the file at path with the given os.FileInfo to the provided TarWriter
func addFileToTar(tw *tar.Writer, path, parentDir string, fi os.FileInfo) error {
	if fi.Mode()&os.ModeSocket != 0 {
		return nil
	}
	header, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	header.Name, err = filepath.Rel(parentDir, path)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		var err error
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		header.Linkname = target
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if fi.Mode().IsRegular() {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
	}
	return nil
}

func addTextToTar(tw *tar.Writer, fileContents []byte, withName string) error {
	hdr := &tar.Header{Name: withName, Mode: 0644, Size: int64(len(fileContents))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(fileContents)
	return err
}

func cleanPath(dest, header string) (string, error) {
	joined := filepath.Join(dest, header)
	if strings.HasPrefix(joined, filepath.Clean(dest)) {
		return joined, nil
	}
	return "", fmt.Errorf("bad filepath: %s", header)
}

func untar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			return nil
		}
		if err != nil {
			return err
		}

		path, err := cleanPath(dest, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			_, err := os.Stat(filepath.Dir(path))
			if os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
					return err
				}
			}

			fh, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(fh, tr); err != nil {
				fh.Close()
				return err
			} // #nosec G110
			fh.Close()
		case tar.TypeSymlink:
			_, err := os.Stat(filepath.Dir(path))
			if os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
					return err
				}
			}

			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown file type in tar %d", hdr.Typeflag)
		}
	}
}
