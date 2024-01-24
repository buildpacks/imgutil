package locallayout

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	registryName "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"golang.org/x/sync/errgroup"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/layout"
)

// Store provides methods for interacting with a docker daemon
// in order to save, delete, and report the presence of images,
// as well as download layers for a given image.
type Store struct {
	// required
	dockerClient DockerClient
	// optional
	onDiskLayers []v1.Layer
}

// DockerClient is subset of client.CommonAPIClient required by this package.
type DockerClient interface {
	ImageHistory(ctx context.Context, image string) ([]image.HistoryResponseItem, error)
	ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error)
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	ImageRemove(ctx context.Context, image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
	ImageSave(ctx context.Context, images []string) (io.ReadCloser, error)
	ImageTag(ctx context.Context, image, ref string) error
	Info(ctx context.Context) (types.Info, error)
	ServerVersion(ctx context.Context) (types.Version, error)
}

var _ imgutil.ImageStore = &Store{}

// images

func (s *Store) Contains(identifier string) bool {
	_, _, err := s.dockerClient.ImageInspectWithRaw(context.Background(), identifier)
	return err == nil
}

func (s *Store) Delete(identifier string) error {
	if !s.Contains(identifier) {
		return nil
	}
	options := types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	}
	_, err := s.dockerClient.ImageRemove(context.Background(), identifier, options)
	return err
}

func (s *Store) Save(image imgutil.IdentifiableV1Image, withName string, withAdditionalNames ...string) (string, error) {
	withName = tryNormalizing(withName)
	identifier, err := image.Identifier()
	if err != nil {
		return "", err
	}

	// save
	inspect, err := s.doSave(image, withName)
	if err != nil {
		if err = s.DownloadLayersFor(identifier.String()); err != nil {
			return "", err
		}
		inspect, err = s.doSave(image, withName)
		if err != nil {
			saveErr := imgutil.SaveError{}
			for _, n := range append([]string{withName}, withAdditionalNames...) {
				saveErr.Errors = append(saveErr.Errors, imgutil.SaveDiagnostic{ImageName: n, Cause: err})
			}
			return "", saveErr
		}
	}

	// tag additional names
	var errs []imgutil.SaveDiagnostic
	for _, n := range append([]string{withName}, withAdditionalNames...) {
		if err = s.dockerClient.ImageTag(context.Background(), inspect.ID, n); err != nil {
			errs = append(errs, imgutil.SaveDiagnostic{ImageName: n, Cause: err})
		}
	}
	if len(errs) > 0 {
		return "", imgutil.SaveError{Errors: errs}
	}

	return inspect.ID, nil
}

func tryNormalizing(name string) string {
	// ensure primary tag is valid
	t, err := registryName.NewTag(name, registryName.WeakValidation)
	if err != nil {
		return name
	}
	return t.Name() // returns valid 'name:tag' appending 'latest', if missing tag
}

func (s *Store) doSave(image v1.Image, withName string) (types.ImageInspect, error) {
	ctx := context.Background()
	done := make(chan error)

	var err error
	pr, pw := io.Pipe()
	defer pw.Close()

	go func() {
		var res types.ImageLoadResponse
		res, err = s.dockerClient.ImageLoad(ctx, pr, true)
		if err != nil {
			done <- err
			return
		}

		// only return the response error after the response is drained and closed
		responseErr := checkResponseError(res.Body)
		drainCloseErr := ensureReaderClosed(res.Body)
		if responseErr != nil {
			done <- responseErr
			return
		}
		if drainCloseErr != nil {
			done <- drainCloseErr
		}

		done <- nil
	}()

	tw := tar.NewWriter(pw)
	defer tw.Close()

	if err = s.addImageToTar(tw, image, withName); err != nil {
		return types.ImageInspect{}, err
	}
	tw.Close()
	pw.Close()
	err = <-done
	if err != nil {
		return types.ImageInspect{}, fmt.Errorf("loading image %q. first error: %w", withName, err)
	}

	inspect, _, err := s.dockerClient.ImageInspectWithRaw(context.Background(), withName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return types.ImageInspect{}, fmt.Errorf("saving image %q: %w", withName, err)
		}
		return types.ImageInspect{}, err
	}
	return inspect, nil
}

func (s *Store) addImageToTar(tw *tar.Writer, image v1.Image, withName string) error {
	path, err := os.MkdirTemp("", "") // TODO: remove?
	if err != nil {
		return err
	}
	layoutPath, err := layout.Write(path, empty.Index)
	if err != nil {
		return err
	}
	if err = layoutPath.AppendImage(image, layout.WithAnnotations(map[string]string{"io.containerd.image.name": withName})); err != nil {
		return err
	}
	if err = AddDirToArchive(tw, path); err != nil {
		return err
	}
	manifestJSON, err := manifestJSONFor(image, withName)
	if err != nil {
		return err
	}
	return addTextToTar(tw, manifestJSON, "manifest.json")
}

func manifestJSONFor(image v1.Image, withName string) ([]byte, error) {
	manifest, err := image.Manifest()
	if err != nil {
		return nil, err
	}
	var layerPaths []string
	for _, layer := range manifest.Layers {
		layerPaths = append(layerPaths, filepath.Join("blobs", layer.Digest.Algorithm, layer.Digest.Hex))
	}
	manifestJSON, err := json.Marshal([]map[string]interface{}{
		{
			"Config":   filepath.Join("blobs", manifest.Config.Digest.Algorithm, manifest.Config.Digest.Hex),
			"RepoTags": []string{withName},
			"Layers":   layerPaths,
		},
	})
	if err != nil {
		return nil, err
	}
	return manifestJSON, nil
}

// FIXME: find a place for these

// AddDirToArchive walks dir writes entries describing dir and all of its children files to the provided *tar.Writer
func AddDirToArchive(tw *tar.Writer, dir string) error {
	dir = filepath.Clean(dir)

	return filepath.Walk(dir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return AddFileToArchive(tw, file, dir, fi)
	})
}

// AddFileToArchive writes an entry describing the file at path with the given os.FileInfo to the provided TarWriter
func AddFileToArchive(tw *tar.Writer, path, parentDir string, fi os.FileInfo) error {
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

func (s *Store) addLayerToTar(tw *tar.Writer, layer v1.Layer) (string, error) {
	layerDiffID, err := layer.DiffID()
	if err != nil {
		return "", err
	}
	withName := fmt.Sprintf("/%s.tar", layerDiffID.String())

	uncompressedSize, err := getLayerSize(layer)
	if err != nil {
		return "", err
	}
	hdr := &tar.Header{Name: withName, Mode: 0644, Size: uncompressedSize}
	if err = tw.WriteHeader(hdr); err != nil {
		return "", err
	}

	layerReader, err := layer.Uncompressed()
	if err != nil {
		return "", err
	}
	defer layerReader.Close()
	if _, err = io.Copy(tw, layerReader); err != nil {
		return "", err
	}

	return withName, nil
}

// FIXME: this is a hack because the daemon expects uncompressed layer size and a v1.Layer reports compressed layer size;
// when we send OCI layout tars we should be able to remove this method and get improved performance
func getLayerSize(layer v1.Layer) (int64, error) {
	layerReader, err := layer.Uncompressed()
	if err != nil {
		return 0, err
	}
	defer layerReader.Close()

	var size int64
	size, err = io.Copy(io.Discard, layerReader)
	if err != nil {
		return 0, err
	}
	return size, nil
}

func addTextToTar(tw *tar.Writer, fileContents []byte, withName string) error {
	hdr := &tar.Header{Name: withName, Mode: 0644, Size: int64(len(fileContents))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(fileContents)
	return err
}

func checkResponseError(r io.Reader) error {
	decoder := json.NewDecoder(r)
	var jsonMessage jsonmessage.JSONMessage
	if err := decoder.Decode(&jsonMessage); err != nil {
		return fmt.Errorf("parsing daemon response: %w", err)
	}

	if jsonMessage.Error != nil {
		return fmt.Errorf("embedded daemon response: %w", jsonMessage.Error)
	}
	return nil
}

// ensureReaderClosed drains and closes and reader, returning the first error
func ensureReaderClosed(r io.ReadCloser) error {
	_, err := io.Copy(io.Discard, r)
	if closeErr := r.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

func (s *Store) SaveFile(image imgutil.IdentifiableV1Image, withName string) (string, error) {
	withName = tryNormalizing(withName)

	f, err := os.CreateTemp("", "imgutil.local.image.export.*.tar")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(f.Name())
		}
	}()

	// All layers need to be present here. Missing layers are either due to utilization of
	// (1) WithPreviousImage(), or (2) FromBaseImage().
	// The former is only relevant if ReuseLayers() has been called which takes care of resolving them.
	// The latter case needs to be handled explicitly.
	identifier, err := image.Identifier()
	if err != nil {
		return "", err
	}
	if err = s.DownloadLayersFor(identifier.String()); err != nil {
		return "", err
	}

	errs, _ := errgroup.WithContext(context.Background())
	pr, pw := io.Pipe()

	// File writer
	errs.Go(func() error {
		defer pr.Close()
		_, err = f.ReadFrom(pr)
		return err
	})

	// Tar producer
	errs.Go(func() error {
		defer pw.Close()

		tw := tar.NewWriter(pw)
		defer tw.Close()

		return s.addImageToTar(tw, image, withName)
	})

	err = errs.Wait()
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

// layers

func (s *Store) DownloadLayersFor(identifier string) error {
	layers, err := downloadLayersFor(identifier, s.dockerClient)
	if err != nil {
		return err
	}
	s.onDiskLayers = append(s.onDiskLayers, layers...)
	return nil
}

func downloadLayersFor(identifier string, dockerClient DockerClient) ([]v1.Layer, error) {
	if identifier == "" {
		return nil, nil
	}
	ctx := context.Background()

	// TODO: there must be a better way to do this
	if len(identifier) > 12 {
		identifier = identifier[:12]
	}

	imageReader, err := dockerClient.ImageSave(ctx, []string{identifier})
	if err != nil {
		return nil, fmt.Errorf("saving base image with ID %q from the docker daemon: %w", identifier, err)
	}
	defer ensureReaderClosed(imageReader)

	tmpDir, err := os.MkdirTemp("", "imgutil.local.image.")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	err = untar(imageReader, tmpDir)
	if err != nil {
		return nil, err
	}

	mf, err := os.Open(filepath.Clean(filepath.Join(tmpDir, "manifest.json")))
	if err != nil {
		return nil, err
	}
	defer mf.Close()

	var manifest []struct {
		Config string
		Layers []string
	}
	if err := json.NewDecoder(mf).Decode(&manifest); err != nil {
		return nil, err
	}
	if len(manifest) != 1 {
		return nil, fmt.Errorf("manifest.json had unexpected number of entries: %d", len(manifest))
	}

	cfg, err := os.Open(filepath.Clean(filepath.Join(tmpDir, manifest[0].Config)))
	if err != nil {
		return nil, err
	}
	defer cfg.Close()
	var configFile struct {
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err = json.NewDecoder(cfg).Decode(&configFile); err != nil {
		return nil, err
	}

	layers := make([]v1.Layer, len(configFile.RootFS.DiffIDs))
	for idx, diffID := range configFile.RootFS.DiffIDs {
		var h v1.Hash
		h, err = v1.NewHash(diffID)
		if err != nil {
			return nil, err
		}
		layer, err := tarball.LayerFromFile(filepath.Join(tmpDir, manifest[0].Layers[idx]))
		if err != nil {
			return nil, err
		}
		layers[idx] = &v1LayerFacade{
			diffID:                  h,
			optionalUnderlyingLayer: layer,
		}
	}
	return layers, nil
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

func cleanPath(dest, header string) (string, error) {
	joined := filepath.Join(dest, header)
	if strings.HasPrefix(joined, filepath.Clean(dest)) {
		return joined, nil
	}
	return "", fmt.Errorf("bad filepath: %s", header)
}

func (s *Store) Layers() []v1.Layer {
	return s.onDiskLayers
}
