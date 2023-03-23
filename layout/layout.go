package layout

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/tarball"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

var _ imgutil.Image = (*Image)(nil)

type Image struct {
	v1.Image
	path       string
	prevLayers []v1.Layer
	createdAt  time.Time
	refName    string // holds org.opencontainers.image.ref.name value
}

// getters

func (i *Image) Architecture() (string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return "", errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return "", fmt.Errorf("missing config for image at path %q", i.path)
	}
	if cfg.Architecture == "" {
		return "", fmt.Errorf("missing Architecture for image at path %q", i.path)
	}
	return cfg.Architecture, nil
}

func (i *Image) CreatedAt() (time.Time, error) {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "getting createdAt time for image at path %q", i.path)
	}
	return configFile.Created.UTC(), nil
}

func (i *Image) Env(key string) (string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return "", errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return "", fmt.Errorf("missing config for image at path %q", i.path)
	}
	for _, envVar := range cfg.Config.Env {
		parts := strings.Split(envVar, "=")
		if parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (i *Image) Entrypoint() ([]string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return nil, errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return nil, fmt.Errorf("missing config for image at path %q", i.path)
	}
	return cfg.Config.Entrypoint, nil
}

// Found tells whether the image exists in the repository by `Name()`.
func (i *Image) Found() bool {
	return ImageExists(i.path)
}

func ImageExists(path string) bool {
	if !pathExists(path) {
		return false
	}
	index := filepath.Join(path, "index.json")
	if _, err := os.Stat(index); os.IsNotExist(err) {
		return false
	}
	return true
}

func pathExists(path string) bool {
	if path != "" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return true
		}
	}
	return false
}

func (i *Image) GetAnnotateRefName() (string, error) {
	return i.refName, nil
}

// GetLayer retrieves layer by diff id. Returns a reader of the uncompressed contents of the layer.
// When the layers (notExistsLayer) came from a sparse image returns an empty reader
func (i *Image) GetLayer(sha string) (io.ReadCloser, error) {
	layers, err := i.Image.Layers()
	if err != nil {
		return nil, err
	}

	layer, err := findLayerWithSha(layers, sha)
	if err != nil {
		return nil, err
	}

	return layer.Uncompressed()
}

// Identifier
// Each image's ID is given by the SHA256 hash of its configuration JSON. It is represented as a hexadecimal encoding of 256 bits,
// e.g., sha256:a9561eb1b190625c9adb5a9513e72c4dedafc1cb2d4c5236c9a6957ec7dfd5a9.
func (i *Image) Identifier() (imgutil.Identifier, error) {
	hash, err := i.Image.Digest()
	if err != nil {
		return nil, errors.Wrapf(err, "getting identifier for image at path %q", i.path)
	}
	return newLayoutIdentifier(i.path, hash)
}

func (i *Image) Label(key string) (string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return "", fmt.Errorf("getting config for image at path %q: %w", i.path, err)
	}
	if cfg == nil {
		return "", fmt.Errorf("missing config for image at path %q", i.path)
	}
	labels := cfg.Config.Labels
	return labels[key], nil
}

func (i *Image) Labels() (map[string]string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return nil, errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return nil, fmt.Errorf("missing config for image at path %q", i.path)
	}
	return cfg.Config.Labels, nil
}

// Layers overrides v1.Image Layers(), because we allow sparse image in OCI layout, sometimes some blobs
// are missing. This method checks:
// If there is data, return the layer
// If there is no data, return a notExistsLayer
func (i *Image) Layers() ([]v1.Layer, error) {
	layers, err := i.Image.Layers()
	if err != nil {
		return nil, err
	}

	var retLayers []v1.Layer
	for pos, layer := range layers {
		if hasData(layer) {
			retLayers = append(retLayers, layer)
		} else {
			cfg, err := i.Image.ConfigFile()
			if err != nil {
				return nil, err
			}
			diffID := cfg.RootFS.DiffIDs[pos]
			retLayers = append(retLayers, &notExistsLayer{Layer: layer, diffID: diffID})
		}
	}
	return retLayers, nil
}

func hasData(layer v1.Layer) bool {
	_, err := layer.Compressed()
	return err == nil
}

type notExistsLayer struct {
	v1.Layer
	diffID v1.Hash
}

func (l *notExistsLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (l *notExistsLayer) DiffID() (v1.Hash, error) {
	return l.diffID, nil
}

func (l *notExistsLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (i *Image) ManifestSize() (int64, error) {
	return i.Image.Size()
}

func (i *Image) Name() string {
	return i.path
}

func (i *Image) OS() (string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return "", errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return "", fmt.Errorf("missing config for image at path %q", i.path)
	}
	if cfg.OS == "" {
		return "", fmt.Errorf("missing OS for image at path %q", i.path)
	}
	return cfg.OS, nil
}

func (i *Image) OSVersion() (string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return "", errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return "", fmt.Errorf("missing config for image at path %q", i.path)
	}
	return cfg.OSVersion, nil
}

func (i *Image) TopLayer() (string, error) {
	all, err := i.Image.Layers()
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", fmt.Errorf("image at path %q has no layers", i.Name())
	}
	topLayer := all[len(all)-1]
	hex, err := topLayer.DiffID()
	if err != nil {
		return "", err
	}
	return hex.String(), nil
}

func (i *Image) Variant() (string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return "", errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return "", fmt.Errorf("missing config for image at path %q", i.path)
	}
	return cfg.Variant, nil
}

func (i *Image) WorkingDir() (string, error) {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return "", errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return "", fmt.Errorf("missing config for image at path %q", i.path)
	}
	return cfg.Config.WorkingDir, nil
}

// setters

func (i *Image) AnnotateRefName(refName string) error {
	i.refName = refName
	return nil
}

func (i *Image) Rename(name string) {
	i.path = name
}

func (i *Image) SetArchitecture(architecture string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	configFile.Architecture = architecture
	err = i.mutateConfigFile(i.Image, configFile)
	return err
}

func (i *Image) SetCmd(cmd ...string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.Cmd = cmd
	err = i.mutateConfig(i.Image, config)
	return err
}

func (i *Image) SetEnv(key string, val string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	ignoreCase := configFile.OS == "windows"
	for idx, e := range config.Env {
		parts := strings.Split(e, "=")
		foundKey := parts[0]
		searchKey := key
		if ignoreCase {
			foundKey = strings.ToUpper(foundKey)
			searchKey = strings.ToUpper(searchKey)
		}
		if foundKey == searchKey {
			config.Env[idx] = fmt.Sprintf("%s=%s", key, val)
			err = i.mutateConfig(i.Image, config)
			return err
		}
	}
	config.Env = append(config.Env, fmt.Sprintf("%s=%s", key, val))
	err = i.mutateConfig(i.Image, config)
	return err
}

func (i *Image) SetEntrypoint(ep ...string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.Entrypoint = ep
	err = i.mutateConfig(i.Image, config)
	return err
}

func (i *Image) SetLabel(key string, val string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[key] = val
	err = i.mutateConfig(i.Image, config)
	if err != nil {
		return errors.Wrapf(err, "set label key=%s value=%s", key, val)
	}
	return nil
}

func (i *Image) SetOS(osVal string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	configFile.OS = osVal
	err = i.mutateConfigFile(i.Image, configFile)
	return err
}

func (i *Image) SetOSVersion(osVersion string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	configFile.OSVersion = osVersion
	err = i.mutateConfigFile(i.Image, configFile)
	return err
}

func (i *Image) SetVariant(variant string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	configFile.Variant = variant
	err = i.mutateConfigFile(i.Image, configFile)
	return err
}

func (i *Image) SetWorkingDir(dir string) error {
	configFile, err := i.Image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.WorkingDir = dir
	err = i.mutateConfig(i.Image, config)
	return err
}

// modifiers

// AddLayer adds an uncompressed tarred layer to the image
func (i *Image) AddLayer(path string) error {
	layer, err := tarball.LayerFromFile(path)
	if err != nil {
		return err
	}
	return i.addOCILayer(layer)
}

// addOCILayer appends the provided layer with media type application/vnd.oci.image.layer.v1.tar+gzip
func (i *Image) addOCILayer(layer v1.Layer) error {
	additions := layersAddendum([]v1.Layer{layer})
	image, err := mutate.Append(i.Image, additions...)
	if err != nil {
		return errors.Wrap(err, "add layer")
	}
	return i.mutateImage(image)
}

// layersAddendum creates an Addendum array with the given layers
// and 'application/vnd.oci.image.layer.v1.tar+gzip' media type
func layersAddendum(layers []v1.Layer) []mutate.Addendum {
	additions := make([]mutate.Addendum, 0)
	for _, layer := range layers {
		additions = append(additions, mutate.Addendum{
			MediaType: types.OCILayer,
			Layer:     layer,
		})
	}
	return additions
}

func (i *Image) AddLayerWithDiffID(path, diffID string) error {
	// this is equivalent to AddLayer in the layout case
	// it exists to provide optimize performance for local images
	return i.AddLayer(path)
}

func (i *Image) Delete() error {
	return os.RemoveAll(i.path)
}

func (i *Image) Rebase(s string, image imgutil.Image) error {
	return errors.New("not yet implemented")
}

func (i *Image) RemoveLabel(key string) error {
	cfg, err := i.Image.ConfigFile()
	if err != nil {
		return errors.Wrapf(err, "getting config file for image at path %q", i.path)
	}
	if cfg == nil {
		return fmt.Errorf("missing config for image at path %q", i.path)
	}
	config := *cfg.Config.DeepCopy()
	delete(config.Labels, key)
	err = i.mutateConfig(i.Image, config)
	return err
}

func (i *Image) ReuseLayer(sha string) error {
	layer, err := findLayerWithSha(i.prevLayers, sha)
	if err != nil {
		return err
	}
	return i.addOCILayer(layer)
}

// helpers

func findLayerWithSha(layers []v1.Layer, diffID string) (v1.Layer, error) {
	for _, layer := range layers {
		dID, err := layer.DiffID()
		if err != nil {
			return nil, errors.Wrap(err, "get diff ID for previous image layer")
		}
		if diffID == dID.String() {
			return layer, nil
		}
	}
	return nil, fmt.Errorf("previous image did not have layer with diff id %q", diffID)
}

// mutateConfig mutates the provided v1.Image to have the provided v1.Config and wraps the result
// into a layout.Image (requires for override methods like Layers()
func (i *Image) mutateConfig(base v1.Image, config v1.Config) error {
	image, err := mutate.Config(base, config)
	if err != nil {
		return err
	}
	return i.mutateImage(image)
}

// mutateConfigFile mutates the provided v1.Image to have the provided v1.ConfigFile and wraps the result
// into a layout.Image (requires for override methods like Layers()
func (i *Image) mutateConfigFile(base v1.Image, configFile *v1.ConfigFile) error {
	image, err := mutate.ConfigFile(base, configFile)
	if err != nil {
		return err
	}
	return i.mutateImage(image)
}

// mutateImage wraps the provided v1.Image into a layout.Image
func (i *Image) mutateImage(base v1.Image) error {
	manifest, err := base.Manifest()
	if err != nil {
		return err
	}
	if validMediaTypes(manifest) {
		i.Image = &Image{
			Image: base,
		}
	} else {
		// images has docker media types, we need to override them
		newBaseImage, err := overrideMediaTypes(base)
		if err != nil {
			return err
		}
		i.Image = &Image{
			Image: newBaseImage,
		}
	}
	return nil
}

// validMediaTypes returns true if media types present in the manifest are the ones defined by the OCI spec
// Docker Media Types will return false.
func validMediaTypes(manifest *v1.Manifest) bool {
	return manifest.MediaType == types.OCIManifestSchema1 &&
		manifest.Config.MediaType == types.OCIConfigJSON
}

// overrideMediaTypes will create a new v1.Image from the provided base image, but replacing
// manifest media type, config media type and layers media type by the ones defined by the OCI spec
func overrideMediaTypes(base v1.Image) (v1.Image, error) {
	config, err := base.ConfigFile()
	if err != nil {
		return nil, err
	}
	config.RootFS.DiffIDs = make([]v1.Hash, 0)

	image := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	image, err = mutate.ConfigFile(image, config)
	if err != nil {
		return nil, err
	}
	image = mutate.ConfigMediaType(image, types.OCIConfigJSON)

	layers, err := base.Layers()
	if err != nil {
		return nil, err
	}

	additions := layersAddendum(layers)
	image, err = mutate.Append(image, additions...)
	if err != nil {
		return nil, err
	}

	return image, nil
}
