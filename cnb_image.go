package imgutil

import (
	"errors"
	"fmt"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

// CNBImageCore wraps a v1.Image and provides most of the methods necessary for the image to satisfy the Image interface.
// Specific implementations may choose to override certain methods, and will need to supply the methods that are omitted,
// such as Identifier() and Found().
// The working image could be any v1.Image,
// but in practice will start off as a pointer to a locallayout.v1ImageFacade (or similar).
type CNBImageCore struct {
	v1.Image // the working image
	Store    ImageStore
	// required
	repoName string
	// optional
	preferredMediaTypes MediaTypes
	preserveHistory     bool
	previousImage       v1.Image
}

type ImageStore interface {
	Contains(identifier string) bool
	Delete(identifier string) error
	Save(image IdentifiableV1Image, withName string, withAdditionalNames ...string) (string, error)
	SaveFile(image IdentifiableV1Image, withName string) (string, error)

	DownloadLayersFor(identifier string) error
	Layers() []v1.Layer
}

type IdentifiableV1Image interface {
	v1.Image
	Identifier() (Identifier, error)
}

var _ v1.Image = &CNBImageCore{}

// FIXME: mark deprecated methods as deprecated on the interface when other packages (remote, layout) expose a v1.Image

// Deprecated: Architecture
func (i *CNBImageCore) Architecture() (string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return "", err
	}
	return configFile.Architecture, nil
}

// Deprecated: CreatedAt
func (i *CNBImageCore) CreatedAt() (time.Time, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return time.Time{}, err
	}
	return configFile.Created.Time, nil
}

// Deprecated: Entrypoint
func (i *CNBImageCore) Entrypoint() ([]string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return nil, err
	}
	return configFile.Config.Entrypoint, nil
}

func (i *CNBImageCore) Env(key string) (string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return "", err
	}
	for _, envVar := range configFile.Config.Env {
		parts := strings.Split(envVar, "=")
		if len(parts) == 2 && parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (i *CNBImageCore) GetAnnotateRefName() (string, error) {
	manifest, err := getManifest(i.Image)
	if err != nil {
		return "", err
	}
	return manifest.Annotations["org.opencontainers.image.ref.name"], nil
}

// Deprecated: History
func (i *CNBImageCore) History() ([]v1.History, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return nil, err
	}
	return configFile.History, nil
}

func (i *CNBImageCore) Kind() string {
	storeType := fmt.Sprintf("%T", i.Store)
	parts := strings.Split(storeType, ".")
	if len(parts) < 2 {
		return storeType
	}
	return strings.TrimPrefix(parts[0], "*")
}

// Deprecated: Label
func (i *CNBImageCore) Label(key string) (string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return "", err
	}
	return configFile.Config.Labels[key], nil
}

// Deprecated: Labels
func (i *CNBImageCore) Labels() (map[string]string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return nil, err
	}
	return configFile.Config.Labels, nil
}

// Deprecated: ManifestSize
func (i *CNBImageCore) ManifestSize() (int64, error) {
	return i.Image.Size()
}

func (i *CNBImageCore) Name() string {
	return i.repoName
}

// Deprecated: OS
func (i *CNBImageCore) OS() (string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return "", err
	}
	return configFile.OS, nil
}

// Deprecated: OSVersion
func (i *CNBImageCore) OSVersion() (string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return "", err
	}
	return configFile.OSVersion, nil
}

func (i *CNBImageCore) TopLayer() (string, error) {
	layers, err := i.Image.Layers()
	if err != nil {
		return "", err
	}
	if len(layers) == 0 {
		return "", fmt.Errorf("image %q has no layers", i.Name())
	}
	topLayer := layers[len(layers)-1]
	hex, err := topLayer.DiffID()
	if err != nil {
		return "", err
	}
	return hex.String(), nil
}

// UnderlyingImage is used to expose a v1.Image from an imgutil.Image, which can be useful in certain situations (such as rebase).
func (i *CNBImageCore) UnderlyingImage() v1.Image {
	return i.Image
}

func (i *CNBImageCore) Valid() bool {
	err := validate.Image(i.Image)
	return err == nil
}

// Deprecated: Variant
func (i *CNBImageCore) Variant() (string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return "", err
	}
	return configFile.Variant, nil
}

// Deprecated: WorkingDir
func (i *CNBImageCore) WorkingDir() (string, error) {
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return "", err
	}
	return configFile.Config.WorkingDir, nil
}

func (i *CNBImageCore) AnnotateRefName(refName string) error {
	manifest, err := getManifest(i.Image)
	if err != nil {
		return err
	}
	manifest.Annotations["org.opencontainers.image.ref.name"] = refName
	mutated := mutate.Annotations(i.Image, manifest.Annotations)
	image, ok := mutated.(v1.Image)
	if !ok {
		return fmt.Errorf("failed to add annotation")
	}
	i.Image = image
	return nil
}

func (i *CNBImageCore) Rename(name string) {
	i.repoName = name
}

// Deprecated: SetArchitecture
func (i *CNBImageCore) SetArchitecture(architecture string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.Architecture = architecture
	})
}

// Deprecated: SetCmd
func (i *CNBImageCore) SetCmd(cmd ...string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.Config.Cmd = cmd
	})
}

// Deprecated: SetEntrypoint
func (i *CNBImageCore) SetEntrypoint(ep ...string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.Config.Entrypoint = ep
	})
}

func (i *CNBImageCore) SetEnv(key, val string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		ignoreCase := c.OS == "windows"
		for idx, e := range c.Config.Env {
			parts := strings.Split(e, "=")
			if len(parts) < 1 {
				continue
			}
			foundKey := parts[0]
			searchKey := key
			if ignoreCase {
				foundKey = strings.ToUpper(foundKey)
				searchKey = strings.ToUpper(searchKey)
			}
			if foundKey == searchKey {
				c.Config.Env[idx] = fmt.Sprintf("%s=%s", key, val)
				return
			}
		}
		c.Config.Env = append(c.Config.Env, fmt.Sprintf("%s=%s", key, val))
	})
}

// Deprecated: SetHistory
func (i *CNBImageCore) SetHistory(histories []v1.History) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.History = histories
	})
}

func (i *CNBImageCore) SetLabel(key, val string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		if c.Config.Labels == nil {
			c.Config.Labels = make(map[string]string)
		}
		c.Config.Labels[key] = val
	})
}

func (i *CNBImageCore) SetOS(osVal string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.OS = osVal
	})
}

// Deprecated: SetOSVersion
func (i *CNBImageCore) SetOSVersion(osVersion string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.OSVersion = osVersion
	})
}

// Deprecated: SetVariant
func (i *CNBImageCore) SetVariant(variant string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.Variant = variant
	})
}

// Deprecated: SetWorkingDir
func (i *CNBImageCore) SetWorkingDir(dir string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.Config.WorkingDir = dir
	})
}

// modifiers

var emptyHistory = v1.History{Created: v1.Time{Time: NormalizedDateTime}}

func (i *CNBImageCore) AddLayer(path string) error {
	return i.AddLayerWithDiffIDAndHistory(path, "ignored", emptyHistory)
}

func (i *CNBImageCore) AddLayerWithDiffID(path, _ string) error {
	return i.AddLayerWithDiffIDAndHistory(path, "ignored", emptyHistory)
}

func (i *CNBImageCore) AddLayerWithDiffIDAndHistory(path, _ string, history v1.History) error {
	layer, err := tarball.LayerFromFile(path)
	if err != nil {
		return err
	}
	if !i.preserveHistory {
		history = emptyHistory
	}
	configFile, err := getConfigFile(i)
	if err != nil {
		return err
	}
	history.Created = configFile.Created
	i.Image, err = mutate.Append(
		i.Image,
		mutate.Addendum{
			Layer:     layer,
			History:   history,
			MediaType: i.preferredMediaTypes.LayerType(),
		},
	)
	return err
}

func (i *CNBImageCore) Rebase(baseTopLayerDiffID string, withNewBase Image) error {
	if i.Kind() != withNewBase.Kind() {
		return fmt.Errorf("expected new base to be a %s image; got %s", i.Kind(), withNewBase.Kind())
	}
	newBase := withNewBase.UnderlyingImage() // FIXME: when all imgutil.Images are v1.Images, we can remove this part
	var err error
	i.Image, err = mutate.Rebase(i.Image, i.newV1ImageFacade(baseTopLayerDiffID), newBase)
	if err != nil {
		return err
	}

	// ensure new config matches provided image
	newBaseConfigFile, err := getConfigFile(newBase)
	if err != nil {
		return err
	}
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		c.Architecture = newBaseConfigFile.Architecture
		c.OS = newBaseConfigFile.OS
		c.OSVersion = newBaseConfigFile.OSVersion
	})
}

func (i *CNBImageCore) newV1ImageFacade(topLayerDiffID string) v1.Image {
	return &v1ImageFacade{
		Image:          i,
		topLayerDiffID: topLayerDiffID,
	}
}

type v1ImageFacade struct {
	v1.Image
	topLayerDiffID string
}

func (si *v1ImageFacade) Layers() ([]v1.Layer, error) {
	all, err := si.Image.Layers()
	if err != nil {
		return nil, err
	}
	for i, l := range all {
		d, err := l.DiffID()
		if err != nil {
			return nil, err
		}
		if d.String() == si.topLayerDiffID {
			return all[0 : i+1], nil
		}
	}
	return nil, errors.New("could not find base layer in image")
}

func (i *CNBImageCore) RemoveLabel(key string) error {
	return i.MutateConfigFile(func(c *v1.ConfigFile) {
		delete(c.Config.Labels, key)
	})
}

func (i *CNBImageCore) ReuseLayer(diffID string) error {
	if i.previousImage == nil {
		return errors.New("failed to reuse layer because no previous image was provided")
	}
	idx, err := getLayerIndex(diffID, i.previousImage)
	if err != nil {
		return err
	}
	previousHistory, err := getHistory(idx, i.previousImage)
	if err != nil {
		return err
	}
	return i.ReuseLayerWithHistory(diffID, previousHistory)
}

func getLayerIndex(forDiffID string, fromImage v1.Image) (int, error) {
	layerHash, err := v1.NewHash(forDiffID)
	if err != nil {
		return -1, err
	}
	configFile, err := getConfigFile(fromImage)
	if err != nil {
		return -1, err
	}
	for idx, configHash := range configFile.RootFS.DiffIDs {
		if layerHash.String() == configHash.String() {
			return idx, nil
		}
	}
	return -1, fmt.Errorf("failed to find diffID %s in config file", layerHash.String())
}

func getHistory(forIndex int, fromImage v1.Image) (v1.History, error) {
	configFile, err := getConfigFile(fromImage)
	if err != nil {
		return v1.History{}, err
	}
	if len(configFile.History) <= forIndex {
		return v1.History{}, fmt.Errorf("wanted history at index %d; history has length %d", forIndex, len(configFile.History))
	}
	return configFile.History[forIndex], nil
}

func (i *CNBImageCore) ReuseLayerWithHistory(diffID string, history v1.History) error {
	layerHash, err := v1.NewHash(diffID)
	if err != nil {
		return err
	}
	layer, err := i.previousImage.LayerByDiffID(layerHash)
	if err != nil {
		return err
	}
	if !i.preserveHistory {
		history = emptyHistory
	}
	i.Image, err = mutate.Append(
		i.Image,
		mutate.Addendum{
			Layer:     layer,
			History:   history,
			MediaType: i.preferredMediaTypes.LayerType(),
		},
	)
	return err
}

// helpers

func (i *CNBImageCore) MutateConfigFile(withFunc func(c *v1.ConfigFile)) error {
	// FIXME: put MutateConfigFile on the interface when `remote` and `layout` packages also support it.
	configFile, err := getConfigFile(i.Image)
	if err != nil {
		return err
	}
	withFunc(configFile)
	i.Image, err = mutate.ConfigFile(i.Image, configFile)
	return err
}

func getConfigFile(image v1.Image) (*v1.ConfigFile, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}
	if configFile == nil {
		return nil, errors.New("missing config file")
	}
	return configFile, nil
}

func getManifest(image v1.Image) (*v1.Manifest, error) {
	manifest, err := image.Manifest()
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		return nil, errors.New("missing manifest")
	}
	return manifest, nil
}
