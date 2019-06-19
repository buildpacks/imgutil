package imgutil

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

type LocalImage struct {
	repoName         string
	docker           *client.Client
	inspect          types.ImageInspect
	layerPaths       []string
	currentTempImage string
	requestGroup     singleflight.Group
	prevName         string
	easyAddLayers    []string
}

type FileSystemLocalImage struct {
	dir       string
	layersMap map[string]string
}

type LocalImageOption func(image *LocalImage) (*LocalImage, error)

func verifyImage(docker *client.Client, imageName string, optional bool) (types.ImageInspect, error) {
	var (
		err     error
		inspect types.ImageInspect
	)

	if inspect, _, err = docker.ImageInspectWithRaw(context.Background(), imageName); err != nil {
		if client.IsErrNotFound(err) {
			if optional {
				return inspect, nil
			} else {
				return inspect, fmt.Errorf("there is no image with name '%s'", imageName)
			}
		}

		return inspect, errors.Wrapf(err, "verifying image '%s'", imageName)
	}

	return inspect, nil
}

func WithPreviousLocalImage(imageName string) LocalImageOption {
	return func(l *LocalImage) (*LocalImage, error) {
		if _, err := verifyImage(l.docker, imageName, true); err != nil {
			return l, err
		}

		l.prevName = imageName

		return l, nil
	}
}

func FromLocalImageBase(imageName string) LocalImageOption {
	return func(l *LocalImage) (*LocalImage, error) {
		var (
			err error
			inspect types.ImageInspect
		)

		if inspect, err = verifyImage(l.docker, imageName, true); err != nil {
			return l, err
		}

		l.inspect = inspect
		l.layerPaths = make([]string, len(l.inspect.RootFS.Layers))

		return l, nil
	}
}

func NewLocalImage(repoName string, dockerClient *client.Client, ops ...LocalImageOption) (Image, error) {
	inspect := types.ImageInspect{}
	inspect.Config = &container.Config{
		Labels: map[string]string{},
	}

	image := &LocalImage{
		docker:     dockerClient,
		repoName:   repoName,
		inspect:    inspect,
		layerPaths: make([]string, len(inspect.RootFS.Layers)),
	}

	var err error
	for _, v := range ops {
		image, err = v(image)
		if err != nil {
			return nil, err
		}
	}

	return image, nil
}

func (l *LocalImage) Label(key string) (string, error) {
	if l.inspect.Config == nil {
		return "", nil
	}
	labels := l.inspect.Config.Labels
	return labels[key], nil
}

func (l *LocalImage) Env(key string) (string, error) {
	if l.inspect.Config == nil {
		return "", nil
	}
	for _, envVar := range l.inspect.Config.Env {
		parts := strings.Split(envVar, "=")
		if parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (l *LocalImage) Rename(name string) {
	l.easyAddLayers = nil
	if prevInspect, _, err := l.docker.ImageInspectWithRaw(context.TODO(), name); err == nil {
		if l.sameBase(prevInspect) {
			l.easyAddLayers = prevInspect.RootFS.Layers[len(l.inspect.RootFS.Layers):]
		}
	}

	l.repoName = name
}

func (l *LocalImage) sameBase(prevInspect types.ImageInspect) bool {
	if len(prevInspect.RootFS.Layers) < len(l.inspect.RootFS.Layers) {
		return false
	}
	for i, baseLayer := range l.inspect.RootFS.Layers {
		if baseLayer != prevInspect.RootFS.Layers[i] {
			return false
		}
	}
	return true
}

func (l *LocalImage) Name() string {
	return l.repoName
}

func (l *LocalImage) Found() bool {
	return l.inspect.ID != ""
}

func (l *LocalImage) Digest() (string, error) {
	if !l.Found() {
		return "", fmt.Errorf("failed to get digest, image '%s' does not exist", l.repoName)
	}
	if len(l.inspect.RepoDigests) == 0 {
		return "", nil
	}
	parts := strings.Split(l.inspect.RepoDigests[0], "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("failed to get digest, image '%s' has malformed digest '%s'", l.repoName, l.inspect.RepoDigests[0])
	}
	return parts[1], nil
}

func (l *LocalImage) CreatedAt() (time.Time, error) {
	createdAtTime := l.inspect.Created
	createdTime, err := time.Parse(time.RFC3339Nano, createdAtTime)

	if err != nil {
		return time.Time{}, err
	}
	return createdTime, nil
}

func (l *LocalImage) Rebase(baseTopLayer string, newBase Image) error {
	ctx := context.Background()

	// FIND TOP LAYER
	keepLayers := -1
	for i, diffID := range l.inspect.RootFS.Layers {
		if diffID == baseTopLayer {
			keepLayers = len(l.inspect.RootFS.Layers) - i - 1
			break
		}
	}
	if keepLayers == -1 {
		return fmt.Errorf("'%s' not found in '%s' during rebase", baseTopLayer, l.repoName)
	}

	// SWITCH BASE LAYERS
	newBaseInspect, _, err := l.docker.ImageInspectWithRaw(ctx, newBase.Name())
	if err != nil {
		return errors.Wrap(err, "analyze read previous image config")
	}
	l.inspect.RootFS.Layers = newBaseInspect.RootFS.Layers
	l.layerPaths = make([]string, len(l.inspect.RootFS.Layers))

	// DOWNLOAD IMAGE
	fsImage, err := l.downloadImageOnce(l.repoName)
	if err != nil {
		return err
	}

	// READ MANIFEST.JSON
	b, err := ioutil.ReadFile(filepath.Join(fsImage.dir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest []struct{ Layers []string }
	if err := json.Unmarshal(b, &manifest); err != nil {
		return err
	}
	if len(manifest) != 1 {
		return fmt.Errorf("expected 1 image received %d", len(manifest))
	}

	// ADD EXISTING LAYERS
	for _, filename := range manifest[0].Layers[(len(manifest[0].Layers) - keepLayers):] {
		if err := l.AddLayer(filepath.Join(fsImage.dir, filename)); err != nil {
			return err
		}
	}

	return nil
}

func (l *LocalImage) SetLabel(key, val string) error {
	if l.inspect.Config == nil {
		return fmt.Errorf("failed to set label, image '%s' does not exist", l.repoName)
	}

	l.inspect.Config.Labels[key] = val
	return nil
}

func (l *LocalImage) SetEnv(key, val string) error {
	if l.inspect.Config == nil {
		return fmt.Errorf("failed to set env var, image '%s' does not exist", l.repoName)
	}
	l.inspect.Config.Env = append(l.inspect.Config.Env, fmt.Sprintf("%s=%s", key, val))
	return nil
}

func (l *LocalImage) SetWorkingDir(dir string) error {
	if l.inspect.Config == nil {
		return fmt.Errorf("failed to set working dir, image '%s' does not exist", l.repoName)
	}
	l.inspect.Config.WorkingDir = dir
	return nil
}

func (l *LocalImage) SetEntrypoint(ep ...string) error {
	if l.inspect.Config == nil {
		return fmt.Errorf("failed to set entrypoint, image '%s' does not exist", l.repoName)
	}
	l.inspect.Config.Entrypoint = ep
	return nil
}

func (l *LocalImage) SetCmd(cmd ...string) error {
	if l.inspect.Config == nil {
		return fmt.Errorf("failed to set cmd, image '%s' does not exist", l.repoName)
	}
	l.inspect.Config.Cmd = cmd
	return nil
}

func (l *LocalImage) TopLayer() (string, error) {
	all := l.inspect.RootFS.Layers

	if len(all) == 0 {
		return "", fmt.Errorf("image '%s' has no layers", l.repoName)
	}

	topLayer := all[len(all)-1]
	return topLayer, nil
}

func (l *LocalImage) GetLayer(sha string) (io.ReadCloser, error) {
	fsImage, err := l.downloadImageOnce(l.repoName)
	if err != nil {
		return nil, err
	}

	layerID, ok := fsImage.layersMap[sha]
	if !ok {
		return nil, fmt.Errorf("image '%s' does not contain layer with diff ID '%s'", l.repoName, sha)
	}
	return os.Open(filepath.Join(fsImage.dir, layerID))
}

func (l *LocalImage) AddLayer(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "AddLayer: open layer: %s", path)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return errors.Wrapf(err, "AddLayer: calculate checksum: %s", path)
	}
	sha := hex.EncodeToString(hasher.Sum(make([]byte, 0, hasher.Size())))

	l.inspect.RootFS.Layers = append(l.inspect.RootFS.Layers, "sha256:"+sha)
	l.layerPaths = append(l.layerPaths, path)
	l.easyAddLayers = nil

	return nil
}

func (l *LocalImage) ReuseLayer(sha string) error {
	if len(l.easyAddLayers) > 0 && l.easyAddLayers[0] == sha {
		l.inspect.RootFS.Layers = append(l.inspect.RootFS.Layers, sha)
		l.layerPaths = append(l.layerPaths, "")
		l.easyAddLayers = l.easyAddLayers[1:]
		return nil
	}

	if l.prevName == "" {
		return errors.New("no previous image provided to reuse layers from")
	}

	fsImage, err := l.downloadImageOnce(l.prevName)
	if err != nil {
		return err
	}

	reuseLayer, ok := fsImage.layersMap[sha]
	if !ok {
		return fmt.Errorf("SHA %s was not found in %s", sha, l.repoName)
	}

	return l.AddLayer(filepath.Join(fsImage.dir, reuseLayer))
}

func (l *LocalImage) Save() (string, error) {
	ctx := context.Background()
	done := make(chan error)

	t, err := name.NewTag(l.repoName, name.WeakValidation)
	if err != nil {
		return "", err
	}
	repoName := t.String()

	pr, pw := io.Pipe()
	defer pw.Close()
	go func() {
		res, err := l.docker.ImageLoad(ctx, pr, true)
		if err != nil {
			done <- err
			return
		}
		defer res.Body.Close()
		io.Copy(ioutil.Discard, res.Body)

		done <- nil
	}()

	tw := tar.NewWriter(pw)
	defer tw.Close()

	configFile, err := l.configFile()
	if err != nil {
		return "", errors.Wrap(err, "generate config file")
	}

	imgID := fmt.Sprintf("%x", sha256.Sum256(configFile))
	if err := addTextToTar(tw, imgID+".json", configFile); err != nil {
		return "", err
	}

	var layerPaths []string
	for _, path := range l.layerPaths {
		if path == "" {
			layerPaths = append(layerPaths, "")
			continue
		}
		layerName := fmt.Sprintf("/%x.tar", sha256.Sum256([]byte(path)))
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if err := addFileToTar(tw, layerName, f); err != nil {
			return "", err
		}
		f.Close()
		layerPaths = append(layerPaths, layerName)

	}

	manifest, err := json.Marshal([]map[string]interface{}{
		{
			"Config":   imgID + ".json",
			"RepoTags": []string{repoName},
			"Layers":   layerPaths,
		},
	})
	if err != nil {
		return "", err
	}

	if err := addTextToTar(tw, "manifest.json", manifest); err != nil {
		return "", err
	}

	tw.Close()
	pw.Close()
	err = <-done

	l.requestGroup.Forget(l.repoName)

	if _, _, err = l.docker.ImageInspectWithRaw(context.Background(), imgID); err != nil {
		if client.IsErrNotFound(err) {
			return "", errors.Wrapf(err, "save image '%s'", l.repoName)
		}
		return "", err
	}

	return imgID, err
}

func (l *LocalImage) configFile() ([]byte, error) {
	imgConfig := map[string]interface{}{
		"os":      "linux",
		"created": time.Now().Format(time.RFC3339),
		"config":  l.inspect.Config,
		"rootfs": map[string][]string{
			"diff_ids": l.inspect.RootFS.Layers,
		},
		"history": make([]struct{}, len(l.inspect.RootFS.Layers)),
	}
	return json.Marshal(imgConfig)
}

func (l *LocalImage) Delete() error {
	if !l.Found() {
		return nil
	}
	options := types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	}
	_, err := l.docker.ImageRemove(context.Background(), l.inspect.ID, options)
	return err
}

func (l *LocalImage) downloadImageOnce(imageName string) (*FileSystemLocalImage, error) {
	v, err, _ := l.requestGroup.Do(imageName, func() (details interface{}, err error) {
		return downloadImage(l.docker, imageName)
	})

	if err != nil {
		return nil, err
	}

	return v.(*FileSystemLocalImage), nil
}

func downloadImage(docker *client.Client, imageName string) (*FileSystemLocalImage, error) {
	ctx := context.Background()

	tarFile, err := docker.ImageSave(ctx, []string{imageName})
	if err != nil {
		return nil, err
	}
	defer tarFile.Close()

	tmpDir, err := ioutil.TempDir("", "imgutil.local.image.")
	if err != nil {
		return nil, errors.Wrap(err, "local reuse-layer create temp dir")
	}

	err = untar(tarFile, tmpDir)
	if err != nil {
		return nil, err
	}

	mf, err := os.Open(filepath.Join(tmpDir, "manifest.json"))
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

	df, err := os.Open(filepath.Join(tmpDir, manifest[0].Config))
	if err != nil {
		return nil, err
	}
	defer df.Close()

	var details struct {
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}

	if err = json.NewDecoder(df).Decode(&details); err != nil {
		return nil, err
	}

	if len(manifest[0].Layers) != len(details.RootFS.DiffIDs) {
		return nil, fmt.Errorf("layers and diff IDs do not match, there are %d layers and %d diffIDs", len(manifest[0].Layers), len(details.RootFS.DiffIDs))
	}

	layersMap := make(map[string]string, len(manifest[0].Layers))
	for i, diffID := range details.RootFS.DiffIDs {
		layerID := manifest[0].Layers[i]
		layersMap[diffID] = layerID
	}

	return &FileSystemLocalImage{
		dir:       tmpDir,
		layersMap: layersMap,
	}, nil
}

func addTextToTar(tw *tar.Writer, name string, contents []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(contents))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(contents)
	return err
}

func addFileToTar(tw *tar.Writer, name string, contents *os.File) error {
	fi, err := contents.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(fi.Size())}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, contents)
	return err
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

		path := filepath.Join(dest, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			_, err := os.Stat(filepath.Dir(path))
			if os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return err
				}
			}

			fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(fh, tr); err != nil {
				fh.Close()
				return err
			}
			fh.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown file type in tar %d", hdr.Typeflag)
		}
	}
}
