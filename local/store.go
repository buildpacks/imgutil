package local

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	registryName "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"
	"golang.org/x/sync/errgroup"

	"github.com/buildpacks/imgutil"
)

// debugLog writes to both stderr and a file for visibility inside containers.
// Remove before merging.
func debugLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, msg)
	f, err := os.OpenFile("/tmp/imgutil-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		fmt.Fprintln(f, msg)
		f.Close()
	}
}

// Store provides methods for interacting with a docker daemon
// in order to save, delete, and report the presence of images,
// as well as download layers for a given image.
type Store struct {
	// required
	dockerClient DockerClient
	// optional
	downloadOnce         *sync.Once
	onDiskLayersByDiffID map[v1.Hash]annotatedLayer
	// grpcConn is a gRPC connection to Docker's containerd content store.
	// Lazily created by doDownloadLayersViaContentStore. Stays alive while
	// contentStoreLayer objects reference the content client.
	grpcConn interface{ Close() error }
}

// DockerClient is subset of client.APIClient required by this package.
type DockerClient interface {
	ImageHistory(ctx context.Context, image string, opts ...client.ImageHistoryOption) (client.ImageHistoryResult, error)
	ImageInspect(ctx context.Context, image string, opts ...client.ImageInspectOption) (client.ImageInspectResult, error)
	ImageLoad(ctx context.Context, input io.Reader, opts ...client.ImageLoadOption) (client.ImageLoadResult, error)
	ImageRemove(ctx context.Context, image string, options client.ImageRemoveOptions) (client.ImageRemoveResult, error)
	ImageSave(ctx context.Context, images []string, opts ...client.ImageSaveOption) (client.ImageSaveResult, error)
	ImageTag(ctx context.Context, options client.ImageTagOptions) (client.ImageTagResult, error)
	Info(ctx context.Context, options client.InfoOptions) (client.SystemInfoResult, error)
	ServerVersion(ctx context.Context, options client.ServerVersionOptions) (client.ServerVersionResult, error)
}

type annotatedLayer struct {
	layer            v1.Layer
	uncompressedSize int64
}

func NewStore(dockerClient DockerClient) *Store {
	return &Store{
		dockerClient:         dockerClient,
		downloadOnce:         &sync.Once{},
		onDiskLayersByDiffID: make(map[v1.Hash]annotatedLayer),
	}
}

// images

func (s *Store) Contains(identifier string) bool {
	_, err := s.dockerClient.ImageInspect(context.Background(), identifier)
	return err == nil
}

func (s *Store) Delete(identifier string) error {
	if !s.Contains(identifier) {
		return nil
	}
	options := client.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	}
	_, err := s.dockerClient.ImageRemove(context.Background(), identifier, options)
	return err
}

func (s *Store) Save(img *Image, withName string, withAdditionalNames ...string) (string, error) {
	withName = tryNormalizing(withName)
	var (
		inspect image.InspectResponse
		err     error
	)

	// save
	saveStart := time.Now()
	canOmitBaseLayers := !usesContainerdStorage(s.dockerClient)
	debugLog("[imgutil] Save: containerd=%v, infoCall=%v", !canOmitBaseLayers, time.Since(saveStart))
	if canOmitBaseLayers {
		// During the first save attempt some layers may be excluded.
		// The docker daemon allows this if the given set of layers already exists in the daemon in the given order.
		inspect, err = s.doSave(img, withName)
	}
	if !canOmitBaseLayers || err != nil {
		ensureStart := time.Now()
		if err = img.ensureLayers(); err != nil {
			return "", err
		}
		debugLog("[imgutil] Save: ensureLayers=%v", time.Since(ensureStart))
		doSaveStart := time.Now()
		inspect, err = s.doSave(img, withName)
		debugLog("[imgutil] Save: doSave=%v", time.Since(doSaveStart))
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
		_, err = s.dockerClient.ImageTag(context.Background(), client.ImageTagOptions{
			Source: inspect.ID,
			Target: n,
		})
		if err != nil {
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

func usesContainerdStorage(docker DockerClient) bool {
	infoResult, err := docker.Info(context.Background(), client.InfoOptions{})
	if err != nil {
		return false
	}

	for _, driverStatus := range infoResult.Info.DriverStatus {
		if driverStatus[0] == "driver-type" && driverStatus[1] == "io.containerd.snapshotter.v1" {
			return true
		}
	}

	return false
}

func (s *Store) doSave(img v1.Image, withName string) (image.InspectResponse, error) {
	ctx := context.Background()
	done := make(chan error)

	var err error
	pr, pw := io.Pipe()
	defer pw.Close()

	go func() {
		res, err := s.dockerClient.ImageLoad(ctx, pr, client.ImageLoadWithQuiet(true))
		if err != nil {
			done <- err
			return
		}

		// only return the response error after the response is drained and closed
		responseErr := checkResponseError(res)
		drainCloseErr := ensureReaderClosed(res)
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

	tarStart := time.Now()
	if err = s.addImageToTar(tw, img, withName); err != nil {
		return image.InspectResponse{}, err
	}
	debugLog("[imgutil] doSave: addImageToTar=%v", time.Since(tarStart))
	tw.Close()
	pw.Close()
	loadStart := time.Now()
	err = <-done
	debugLog("[imgutil] doSave: ImageLoad drain=%v", time.Since(loadStart))
	if err != nil {
		return image.InspectResponse{}, fmt.Errorf("loading image %q. first error: %w", withName, err)
	}

	inspect, err := s.dockerClient.ImageInspect(context.Background(), withName)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return image.InspectResponse{}, fmt.Errorf("saving image %q: %w", withName, err)
		}
		return image.InspectResponse{}, err
	}
	return inspect.InspectResponse, nil
}

func (s *Store) addImageToTar(tw *tar.Writer, img v1.Image, withName string) error {
	rawConfigFile, err := img.RawConfigFile()
	if err != nil {
		return err
	}
	configHash := fmt.Sprintf("%x", sha256.Sum256(rawConfigFile))
	if err = addTextToTar(tw, rawConfigFile, configHash+".json"); err != nil {
		return err
	}
	layers, err := img.Layers()
	if err != nil {
		return err
	}
	var (
		layerPaths []string
		blankIdx   int
	)
	for _, layer := range layers {
		layerName, err := s.addLayerToTar(tw, layer, blankIdx)
		if err != nil {
			return err
		}
		blankIdx++
		layerPaths = append(layerPaths, layerName)
	}

	manifestJSON, err := json.Marshal([]map[string]interface{}{
		{
			"Config":   configHash + ".json",
			"RepoTags": []string{withName},
			"Layers":   layerPaths,
		},
	})
	if err != nil {
		return err
	}
	return addTextToTar(tw, manifestJSON, "manifest.json")
}

func (s *Store) addLayerToTar(tw *tar.Writer, layer v1.Layer, blankIdx int) (string, error) {
	layerStart := time.Now()
	// Open the uncompressed reader first. For facade layers backed by a previous image,
	// this triggers lazy download of all previous image layers as a side effect.
	layerReader, err := layer.Uncompressed()
	if err != nil {
		return "", err
	}
	defer layerReader.Close()

	size, err := layer.Size()
	if err != nil {
		return "", err
	}
	if size == -1 { // it's a base (always empty) layer
		layerName := fmt.Sprintf("blank_%d", blankIdx)
		hdr := &tar.Header{Name: layerName, Mode: 0644, Size: 0}
		return layerName, tw.WriteHeader(hdr)
	}
	// it's a populated layer
	layerDiffID, err := layer.DiffID()
	if err != nil {
		return "", err
	}
	layerName := fmt.Sprintf("/%s.tar", layerDiffID.String())

	uncompressedSize := s.getLayerSize(layer)
	if uncompressedSize != -1 {
		// Size is known: write directly from the already-open reader
		hdr := &tar.Header{Name: layerName, Mode: 0644, Size: uncompressedSize}
		if err = tw.WriteHeader(hdr); err != nil {
			return "", err
		}
		writeStart := time.Now()
		written, err := io.Copy(tw, layerReader)
		if err != nil {
			return "", err
		}
		debugLog("[imgutil] addLayerToTar: %s write=%v (%d bytes, size_known=true)",
			layerName, time.Since(writeStart), written)
		return layerName, nil
	}

	// Size is unknown (e.g., content store layer or compressed layer from containerd):
	// decompress to a temp file in a single pass to determine the size,
	// then write the tar header and stream from the temp file.
	// This avoids a second decompression that was previously done in getLayerSize.
	writeStart := time.Now()
	name, err := s.addUnknownSizeLayerToTar(tw, layerReader, layerName)
	debugLog("[imgutil] addLayerToTar: %s total=%v (size_known=false, single_pass)",
		layerName, time.Since(layerStart))
	_ = writeStart
	return name, err
}

func (s *Store) addUnknownSizeLayerToTar(tw *tar.Writer, layerReader io.Reader, layerName string) (string, error) {
	tmpFile, err := os.CreateTemp("", "imgutil.local.layer.")
	if err != nil {
		return "", fmt.Errorf("creating temp file for layer: %w", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	uncompressedSize, err := io.Copy(tmpFile, layerReader)
	if err != nil {
		return "", fmt.Errorf("writing layer to temp file: %w", err)
	}

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seeking temp file: %w", err)
	}

	hdr := &tar.Header{Name: layerName, Mode: 0644, Size: uncompressedSize}
	if err = tw.WriteHeader(hdr); err != nil {
		return "", err
	}
	if _, err = io.Copy(tw, tmpFile); err != nil {
		return "", err
	}
	return layerName, nil
}

// getLayerSize returns the known uncompressed layer size, or -1 if unknown.
// When -1 is returned, the caller uses addUnknownSizeLayerToTar to compute
// the size during writing rather than decompressing the layer a second time.
func (s *Store) getLayerSize(layer v1.Layer) int64 {
	diffID, err := layer.DiffID()
	if err != nil {
		return -1
	}
	knownLayer, layerFound := s.onDiskLayersByDiffID[diffID]
	if layerFound && knownLayer.uncompressedSize != -1 {
		return knownLayer.uncompressedSize
	}
	return -1
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
	var jsonMessage jsonstream.Message
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

func (s *Store) SaveFile(image *Image, withName string) (string, error) {
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
	if err = image.ensureLayers(); err != nil {
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

func (s *Store) downloadLayersFor(identifier string) error {
	var err error
	s.downloadOnce.Do(func() {
		if usesContainerdStorage(s.dockerClient) {
			err = s.doDownloadLayersViaContentStore(identifier)
			if err != nil {
				debugLog("[imgutil] content store path failed (%v), falling back to ImageSave", err)
				err = s.doDownloadLayersFor(identifier)
			}
		} else {
			err = s.doDownloadLayersFor(identifier)
		}
	})
	return err
}

func (s *Store) doDownloadLayersFor(identifier string) error {
	if identifier == "" {
		return nil
	}
	debugLog("[imgutil] doDownloadLayersFor: identifier=%s", identifier)
	ctx := context.Background()

	t0 := time.Now()
	imageReader, err := s.dockerClient.ImageSave(ctx, []string{identifier})
	if err != nil {
		return fmt.Errorf("saving image with ID %q from the docker daemon: %w", identifier, err)
	}
	defer ensureReaderClosed(imageReader)
	debugLog("[imgutil] doDownloadLayersFor: ImageSave open=%v", time.Since(t0))

	tmpDir, err := os.MkdirTemp("", "imgutil.local.image.")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	t1 := time.Now()
	err = untar(imageReader, tmpDir)
	if err != nil {
		return err
	}
	debugLog("[imgutil] doDownloadLayersFor: untar=%v", time.Since(t1))

	// Log extracted files for debugging
	var extractedFiles []string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(tmpDir, path)
		extractedFiles = append(extractedFiles, fmt.Sprintf("%s(%d)", rel, info.Size()))
		return nil
	})
	debugLog("[imgutil] doDownloadLayersFor: extracted files: %v", extractedFiles)

	mf, err := os.Open(filepath.Clean(filepath.Join(tmpDir, "manifest.json")))
	if err != nil {
		return err
	}
	defer mf.Close()

	var manifest []struct {
		Config string
		Layers []string
	}
	if err := json.NewDecoder(mf).Decode(&manifest); err != nil {
		return err
	}
	if len(manifest) != 1 {
		return fmt.Errorf("manifest.json had unexpected number of entries: %d", len(manifest))
	}

	cfg, err := os.Open(filepath.Clean(filepath.Join(tmpDir, manifest[0].Config)))
	if err != nil {
		return err
	}
	defer cfg.Close()
	var configFile struct {
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err = json.NewDecoder(cfg).Decode(&configFile); err != nil {
		return err
	}

	t2 := time.Now()
	for idx := range configFile.RootFS.DiffIDs {
		layerPath := filepath.Join(tmpDir, manifest[0].Layers[idx])
		if _, err := s.AddLayer(layerPath); err != nil {
			return err
		}
	}
	debugLog("[imgutil] doDownloadLayersFor: AddLayer(x%d)=%v", len(configFile.RootFS.DiffIDs), time.Since(t2))
	return nil
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
			defer fh.Close()
			_, err = io.Copy(fh, tr) // #nosec G110
			if err != nil {
				return err
			}
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

func (s *Store) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	layer := s.findLayer(h)
	if layer == nil {
		return nil, fmt.Errorf("failed to find layer with diff ID %q", h.String())
	}
	return layer, nil
}

func (s *Store) findLayer(withHash v1.Hash) v1.Layer {
	aLayer, layerFound := s.onDiskLayersByDiffID[withHash]
	if !layerFound {
		return nil
	}
	return aLayer.layer
}

func (s *Store) AddLayer(fromPath string) (v1.Layer, error) {
	layer, err := tarball.LayerFromFile(fromPath)
	if err != nil {
		return nil, err
	}
	diffID, err := layer.DiffID()
	if err != nil {
		return nil, err
	}
	var uncompressedSize int64
	fileSize, err := func() (int64, error) {
		fi, err := os.Stat(fromPath)
		if err != nil {
			return -1, err
		}
		return fi.Size(), nil
	}()
	if err != nil {
		return nil, err
	}
	compressedSize, err := layer.Size()
	if err != nil {
		return nil, err
	}
	if fileSize == compressedSize {
		// the layer is compressed, we don't know the uncompressed size
		uncompressedSize = -1
	} else {
		uncompressedSize = fileSize
	}
	s.onDiskLayersByDiffID[diffID] = annotatedLayer{
		layer:            layer,
		uncompressedSize: uncompressedSize,
	}
	return layer, nil
}
