package fakes

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	registryName "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

func NewImage(name, topLayerSha string, identifier imgutil.Identifier) *Image {
	return &Image{
		labels:           nil,
		env:              map[string]string{},
		topLayerSha:      topLayerSha,
		identifier:       identifier,
		name:             name,
		cmd:              []string{"initialCMD"},
		layersMap:        map[string]string{},
		prevLayersMap:    map[string]string{},
		createdAt:        time.Now(),
		savedNames:       map[string]bool{},
		os:               "linux",
		osVersion:        "",
		architecture:     "amd64",
		savedAnnotations: map[string]string{},
	}
}

var ErrLayerNotFound = errors.New("layer with given diff id not found")

type Image struct {
	deleted                    bool
	layers                     []string
	history                    []v1.History
	layersMap                  map[string]string
	prevLayersMap              map[string]string
	reusedLayers               []string
	labels                     map[string]string
	env                        map[string]string
	topLayerSha                string
	os                         string
	osVersion                  string
	architecture               string
	variant                    string
	identifier                 imgutil.Identifier
	name                       string
	entryPoint                 []string
	cmd                        []string
	base                       string
	createdAt                  time.Time
	layerDir                   string
	workingDir                 string
	savedNames                 map[string]bool
	manifestSize               int64
	refName                    string
	savedAnnotations           map[string]string
	features, osFeatures, urls []string
}

func mapToStringSlice(data map[string]string) []string {
	var stringSlice []string
	for key, value := range data {
		keyValue := fmt.Sprintf("%s=%s", key, value)
		stringSlice = append(stringSlice, keyValue)
	}
	return stringSlice
}

// ConfigFile implements v1.Image.
func (i *Image) ConfigFile() (*v1.ConfigFile, error) {
	var hashes = make([]v1.Hash, 0)

	for _, layer := range i.layers {
		hash, err := v1.NewHash(layer)
		if err != nil {
			return nil, err
		}

		hashes = append(hashes, hash)
	}
	return &v1.ConfigFile{
		Architecture:  i.architecture,
		OS:            i.os,
		OSVersion:     i.osVersion,
		Variant:       i.variant,
		OSFeatures:    i.osFeatures,
		History:       i.history,
		Created:       v1.Time{Time: i.createdAt},
		Author:        "buildpacks",
		Container:     "containerd",
		DockerVersion: "25.0",
		RootFS: v1.RootFS{
			DiffIDs: hashes,
		},
		Config: v1.Config{
			Cmd:         i.cmd,
			Env:         mapToStringSlice(i.env),
			ArgsEscaped: true,
			Image:       i.identifier.String(),
			WorkingDir:  i.workingDir,
			Labels:      i.labels,
			User:        "cnb",
		},
	}, nil
}

// ConfigName implements v1.Image.
func (i *Image) ConfigName() (v1.Hash, error) {
	c, err := i.ConfigFile()
	if err != nil {
		return v1.Hash{}, err
	}

	return v1.NewHash(c.Config.Image)
}

// LayerByDiffID implements v1.Image.
func (i *Image) LayerByDiffID(hash v1.Hash) (v1.Layer, error) {
	c, err := i.ConfigFile()
	if err != nil {
		return nil, err
	}

	for _, diffID := range c.RootFS.DiffIDs {
		if hash == diffID {
			return Layer(1024, types.DockerLayer, WithHash(hash))
		}
	}

	return nil, ErrLayerNotFound
}

// LayerByDigest implements v1.Image.
func (i *Image) LayerByDigest(hash v1.Hash) (v1.Layer, error) {
	for _, layer := range i.layers {
		if h, err := v1.NewHash(layer); err == nil {
			return Layer(1024, types.DockerLayer, WithHash(h))
		}
	}

	return nil, ErrLayerNotFound
}

// Layers implements v1.Image.
func (i *Image) Layers() (layers []v1.Layer, err error) {
	for _, layer := range i.layers {
		hash, err := v1.NewHash(layer)
		if err != nil {
			return nil, err
		}

		l, err := Layer(1024, types.DockerLayer, WithHash(hash))
		if err != nil {
			return layers, err
		}
		layers = append(layers, l)
	}

	return layers, err
}

type FakeConfigFile struct {
	v1.ConfigFile
}

func NewFakeConfigFile(config v1.ConfigFile) FakeConfigFile {
	return FakeConfigFile{
		ConfigFile: config,
	}
}

func (c FakeConfigFile) RawManifest() ([]byte, error) {
	return json.Marshal(c.ConfigFile)
}

type FakeManifest struct {
	v1.Manifest
}

func NewFakeManifest(mfest v1.Manifest) FakeManifest {
	return FakeManifest{
		Manifest: mfest,
	}
}

func (c FakeManifest) RawManifest() ([]byte, error) {
	return json.Marshal(c.Manifest)
}

func (i *Image) ConfigFileToV1Desc(config v1.ConfigFile) (desc v1.Descriptor, err error) {
	fakeConfig := NewFakeConfigFile(config)
	size, err := partial.Size(fakeConfig)
	if err != nil {
		return desc, err
	}

	digest, err := partial.Digest(fakeConfig)
	if err != nil {
		return desc, err
	}

	return v1.Descriptor{
		MediaType:   types.DockerConfigJSON,
		Size:        size,
		Digest:      digest,
		URLs:        i.urls,
		Annotations: i.savedAnnotations,
		Platform: &v1.Platform{
			OS:           i.os,
			Architecture: i.architecture,
			Variant:      i.variant,
			OSVersion:    i.osVersion,
			Features:     i.features,
			OSFeatures:   i.osFeatures,
		},
	}, nil
}

// Manifest implements v1.Image.
func (i *Image) Manifest() (*v1.Manifest, error) {
	layers, err := i.Layers()
	if err != nil {
		return nil, err
	}

	var layerDesc = make([]v1.Descriptor, 0)
	for _, layer := range layers {
		desc := v1.Descriptor{}
		if desc.Digest, err = layer.Digest(); err != nil {
			return nil, err
		}

		if desc.MediaType, err = layer.MediaType(); err != nil {
			return nil, err
		}

		if desc.Size, err = layer.Size(); err != nil {
			return nil, err
		}

		layerDesc = append(layerDesc, desc)
	}

	cfgFile, err := i.ConfigFile()
	if err != nil {
		return nil, err
	}

	configDesc, err := i.ConfigFileToV1Desc(*cfgFile)
	if err != nil {
		return nil, err
	}

	manifest := &v1.Manifest{
		SchemaVersion: 1,
		MediaType:     types.DockerManifestList,
		Layers:        layerDesc,
		Config:        configDesc,
		Subject:       &configDesc,
		Annotations:   i.savedAnnotations,
	}

	return manifest, nil
}

// RawConfigFile implements v1.Image.
func (i *Image) RawConfigFile() ([]byte, error) {
	config, err := i.ConfigFile()
	if err != nil {
		return nil, err
	}

	return json.Marshal(config)
}

// RawManifest implements v1.Image.
func (i *Image) RawManifest() ([]byte, error) {
	mfest, err := i.Manifest()
	if err != nil {
		return nil, err
	}

	return json.Marshal(mfest)
}

// Size implements v1.Image.
func (i *Image) Size() (int64, error) {
	mfest, err := i.Manifest()
	if err != nil {
		return 0, err
	}
	if mfest == nil {
		return 0, imgutil.ErrManifestUndefined
	}

	return partial.Size(NewFakeManifest(*mfest))
}

func (i *Image) CreatedAt() (time.Time, error) {
	return i.createdAt, nil
}

func (i *Image) History() ([]v1.History, error) {
	return i.history, nil
}

func (i *Image) Label(key string) (string, error) {
	return i.labels[key], nil
}

func (i *Image) Labels() (map[string]string, error) {
	copiedLabels := make(map[string]string)
	for i, l := range i.labels {
		copiedLabels[i] = l
	}
	return copiedLabels, nil
}

func (i *Image) OS() (string, error) {
	return i.os, nil
}

func (i *Image) OSVersion() (string, error) {
	return i.osVersion, nil
}

func (i *Image) Architecture() (string, error) {
	return i.architecture, nil
}

func (i *Image) Variant() (string, error) {
	return i.variant, nil
}

func (i *Image) Features() ([]string, error) {
	return i.features, nil
}

func (i *Image) OSFeatures() ([]string, error) {
	return i.osFeatures, nil
}

func (i *Image) URLs() ([]string, error) {
	return i.urls, nil
}

func (i *Image) Annotations() (map[string]string, error) {
	return i.savedAnnotations, nil
}

func (i *Image) Rename(name string) {
	i.name = name
}

func (i *Image) Name() string {
	return i.name
}

func (i *Image) Identifier() (imgutil.Identifier, error) {
	return i.identifier, nil
}

func (i *Image) Digest() (v1.Hash, error) {
	return v1.Hash{}, nil
}

func (i *Image) MediaType() (types.MediaType, error) {
	return types.MediaType(""), nil
}

func (i *Image) Kind() string {
	return ""
}

func (i *Image) UnderlyingImage() v1.Image {
	return nil
}

func (i *Image) Rebase(_ string, newBase imgutil.Image) error {
	i.base = newBase.Name()
	return nil
}

func (i *Image) SetLabel(k string, v string) error {
	if i.labels == nil {
		i.labels = map[string]string{}
	}
	i.labels[k] = v
	return nil
}

func (i *Image) RemoveLabel(key string) error {
	delete(i.labels, key)
	return nil
}

func (i *Image) SetEnv(k string, v string) error {
	i.env[k] = v
	return nil
}

func (i *Image) SetHistory(history []v1.History) error {
	i.history = history
	return nil
}

func (i *Image) SetOS(o string) error {
	i.os = o
	return nil
}

func (i *Image) SetOSVersion(v string) error {
	i.osVersion = v
	return nil
}

func (i *Image) SetArchitecture(a string) error {
	i.architecture = a
	return nil
}

func (i *Image) SetVariant(a string) error {
	i.variant = a
	return nil
}

func (i *Image) SetFeatures(features []string) error {
	i.features = append(i.features, features...)
	return nil
}

func (i *Image) SetOSFeatures(osFeatures []string) error {
	i.osFeatures = append(i.osFeatures, osFeatures...)
	return nil
}

func (i *Image) SetURLs(urls []string) error {
	i.urls = append(i.urls, urls...)
	return nil
}

func (i *Image) SetAnnotations(annos map[string]string) error {
	if len(i.savedAnnotations) < 1 {
		i.savedAnnotations = make(map[string]string)
	}

	for k, v := range annos {
		i.savedAnnotations[k] = v
	}
	return nil
}

func (i *Image) SetWorkingDir(dir string) error {
	i.workingDir = dir
	return nil
}

func (i *Image) SetEntrypoint(v ...string) error {
	i.entryPoint = v
	return nil
}

func (i *Image) SetCmd(v ...string) error {
	i.cmd = v
	return nil
}

func (i *Image) SetCreatedAt(t time.Time) error {
	i.createdAt = t
	return nil
}

func (i *Image) Env(k string) (string, error) {
	return i.env[k], nil
}

func (i *Image) TopLayer() (string, error) {
	return i.topLayerSha, nil
}

func (i *Image) AddLayer(path string) error {
	sha, err := shaForFile(path)
	if err != nil {
		return err
	}

	i.layersMap["sha256:"+sha] = path
	i.layers = append(i.layers, path)
	i.history = append(i.history, v1.History{})
	return nil
}

func (i *Image) AddLayerWithDiffID(path string, diffID string) error {
	i.layersMap[diffID] = path
	i.layers = append(i.layers, path)
	i.history = append(i.history, v1.History{})
	return nil
}

func (i *Image) AddLayerWithDiffIDAndHistory(path, diffID string, history v1.History) error {
	i.layersMap[diffID] = path
	i.layers = append(i.layers, path)
	i.history = append(i.history, history)
	return nil
}

func shaForFile(path string) (string, error) {
	rc, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", errors.Wrapf(err, "failed to open file")
	}
	defer rc.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, rc); err != nil {
		return "", errors.Wrapf(err, "failed to copy rc to hasher")
	}

	return hex.EncodeToString(hasher.Sum(make([]byte, 0, hasher.Size()))), nil
}

func (i *Image) GetLayer(sha string) (io.ReadCloser, error) {
	path, ok := i.layersMap[sha]
	if !ok {
		return nil, fmt.Errorf("failed to get layer with sha '%s'", sha)
	}

	return os.Open(filepath.Clean(path))
}

func (i *Image) ReuseLayer(sha string) error {
	prevLayer, ok := i.prevLayersMap[sha]
	if !ok {
		return fmt.Errorf("image does not have previous layer with sha '%s'", sha)
	}
	i.reusedLayers = append(i.reusedLayers, sha)
	i.layersMap[sha] = prevLayer
	return nil
}

func (i *Image) ReuseLayerWithHistory(sha string, history v1.History) error {
	if err := i.ReuseLayer(sha); err != nil {
		return err
	}
	i.history = append(i.history, history)
	return nil
}

func (i *Image) Save(additionalNames ...string) error {
	return i.SaveAs(i.Name(), additionalNames...)
}

func (i *Image) SaveAs(name string, additionalNames ...string) error {
	var err error
	i.layerDir, err = os.MkdirTemp("", "fake-image")
	if err != nil {
		return err
	}

	for sha, path := range i.layersMap {
		newPath := filepath.Join(i.layerDir, filepath.Base(path))
		i.copyLayer(path, newPath) // errcheck ignore
		i.layersMap[sha] = newPath
	}

	for l := range i.layers {
		layerPath := i.layers[l]
		i.layers[l] = filepath.Join(i.layerDir, filepath.Base(layerPath))
	}

	allNames := append([]string{name}, additionalNames...)
	if i.refName != "" {
		i.savedAnnotations["org.opencontainers.image.ref.name"] = i.refName
	}

	var errs []imgutil.SaveDiagnostic
	for _, n := range allNames {
		_, err := registryName.ParseReference(n, registryName.WeakValidation)
		if err != nil {
			errs = append(errs, imgutil.SaveDiagnostic{ImageName: n, Cause: err})
		} else {
			i.savedNames[n] = true
		}
	}

	if len(errs) > 0 {
		return imgutil.SaveError{Errors: errs}
	}

	return nil
}

func (i *Image) SaveFile() (string, error) {
	return "", errors.New("not yet implemented")
}

func (i *Image) copyLayer(path, newPath string) error {
	src, err := os.Open(filepath.Clean(path))
	if err != nil {
		return errors.Wrap(err, "opening layer during copy")
	}
	defer src.Close()

	dst, err := os.Create(newPath)
	if err != nil {
		return errors.Wrap(err, "creating new layer during copy")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return errors.Wrap(err, "copying layers")
	}

	return nil
}

func (i *Image) Delete() error {
	i.deleted = true
	return nil
}

func (i *Image) Found() bool {
	return !i.deleted
}

func (i *Image) Valid() bool {
	return !i.deleted
}

func (i *Image) AnnotateRefName(refName string) error {
	i.refName = refName
	return nil
}

func (i *Image) GetAnnotateRefName() (string, error) {
	return i.refName, nil
}

// test methods

func (i *Image) SetIdentifier(identifier imgutil.Identifier) {
	i.identifier = identifier
}

func (i *Image) Cleanup() error {
	return os.RemoveAll(i.layerDir)
}

func (i *Image) AppLayerPath() string {
	return i.layers[0]
}

func (i *Image) Entrypoint() ([]string, error) {
	return i.entryPoint, nil
}

func (i *Image) Cmd() ([]string, error) {
	return i.cmd, nil
}

func (i *Image) ConfigLayerPath() string {
	return i.layers[1]
}

func (i *Image) ReusedLayers() []string {
	return i.reusedLayers
}

func (i *Image) WorkingDir() (string, error) {
	return i.workingDir, nil
}

func (i *Image) AddPreviousLayer(sha, path string) {
	i.prevLayersMap[sha] = path
}

func (i *Image) FindLayerWithPath(path string) (string, error) {
	// we iterate backwards over the layer array b/c later layers could replace a file with a given path
	for idx := len(i.layers) - 1; idx >= 0; idx-- {
		tarPath := i.layers[idx]
		rc, err := os.Open(filepath.Clean(tarPath))
		if err != nil {
			return "", errors.Wrapf(err, "opening layer file '%s'", tarPath)
		}
		defer rc.Close()

		tr := tar.NewReader(rc)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				return "", errors.Wrap(err, "finding next header in layer")
			}

			if header.Name == path {
				return tarPath, nil
			}
		}
	}
	return "", fmt.Errorf("could not find '%s' in any layer.\n\n%s", path, i.tarContents())
}

func (i *Image) tarContents() string {
	var strBuilder = &strings.Builder{}
	strBuilder.WriteString("Layers\n-------\n")
	for idx, tarPath := range i.layers {
		i.writeLayerContents(strBuilder, tarPath)
		if idx < len(i.layers)-1 {
			strBuilder.WriteString("\n")
		}
	}
	return strBuilder.String()
}

func (i *Image) writeLayerContents(strBuilder *strings.Builder, tarPath string) {
	strBuilder.WriteString(fmt.Sprintf("%s\n", filepath.Base(tarPath)))

	rc, err := os.Open(filepath.Clean(tarPath))
	if err != nil {
		strBuilder.WriteString(fmt.Sprintf("Error reading layer files: %s\n", err))
		return
	}
	defer rc.Close()

	tr := tar.NewReader(rc)

	hasFiles := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			if !hasFiles {
				strBuilder.WriteString("  (empty)\n")
			}
			break
		}

		var typ = "F"
		var extra = ""
		switch header.Typeflag {
		case tar.TypeDir:
			typ = "D"
		case tar.TypeSymlink:
			typ = "S"
			extra = fmt.Sprintf(" -> %s", header.Linkname)
		}

		strBuilder.WriteString(fmt.Sprintf("  - [%s] %s%s\n", typ, header.Name, extra))
		hasFiles = true
	}
}

func (i *Image) NumberOfAddedLayers() int {
	return len(i.layers)
}

func (i *Image) IsSaved() bool {
	return len(i.savedNames) > 0
}

func (i *Image) Base() string {
	return i.base
}

func (i *Image) SavedNames() []string {
	var names []string
	for k := range i.savedNames {
		names = append(names, k)
	}

	return names
}

func (i *Image) SetManifestSize(size int64) {
	i.manifestSize = size
}

func (i *Image) ManifestSize() (int64, error) {
	return i.manifestSize, nil
}

func (i *Image) SavedAnnotations() map[string]string {
	return i.savedAnnotations
}

// uncompressedLayer implements partial.UncompressedLayer from raw bytes.
type uncompressedLayer struct {
	diffID    v1.Hash
	mediaType types.MediaType
	content   []byte
}

// DiffID implements partial.UncompressedLayer
func (ul *uncompressedLayer) DiffID() (v1.Hash, error) {
	return ul.diffID, nil
}

// Uncompressed implements partial.UncompressedLayer
func (ul *uncompressedLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBuffer(ul.content)), nil
}

// MediaType returns the media type of the layer
func (ul *uncompressedLayer) MediaType() (types.MediaType, error) {
	return ul.mediaType, nil
}

var _ partial.UncompressedLayer = (*uncompressedLayer)(nil)

// Image returns a pseudo-randomly generated Image.
func V1Image(byteSize, layers int64, options ...Option) (v1.Image, error) {
	adds := make([]mutate.Addendum, 0, 5)
	for i := int64(0); i < layers; i++ {
		layer, err := Layer(byteSize, types.DockerLayer, options...)
		if err != nil {
			return nil, err
		}
		adds = append(adds, mutate.Addendum{
			Layer: layer,
			History: v1.History{
				Author:    "random.Image",
				Comment:   fmt.Sprintf("this is a random history %d of %d", i, layers),
				CreatedBy: "random",
			},
		})
	}

	return mutate.Append(empty.Image, adds...)
}
