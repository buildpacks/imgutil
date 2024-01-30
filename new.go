package imgutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil/layer"
)

func NewCNBImage(options ImageOptions) (*CNBImageCore, error) {
	image := &CNBImageCore{
		Image:               options.BaseImage, // the working image
		createdAt:           getCreatedAt(options),
		preferredMediaTypes: GetPreferredMediaTypes(options),
		preserveHistory:     options.PreserveHistory,
		previousImage:       options.PreviousImage,
	}

	// ensure base image
	var err error
	if image.Image == nil {
		image.Image, err = emptyV1(options.Platform, image.preferredMediaTypes)
		if err != nil {
			return nil, err
		}
	}

	// FIXME: we can call EnsureMediaTypesAndLayers here when locallayout supports replacing the underlying image

	// ensure windows
	if err = prepareNewWindowsImageIfNeeded(image); err != nil {
		return nil, err
	}

	// set config if requested
	if options.Config != nil {
		if err = image.MutateConfigFile(func(c *v1.ConfigFile) {
			c.Config = *options.Config
		}); err != nil {
			return nil, err
		}
	}

	return image, nil
}

func getCreatedAt(options ImageOptions) time.Time {
	if !options.CreatedAt.IsZero() {
		return options.CreatedAt
	}
	return NormalizedDateTime
}

var NormalizedDateTime = time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)

func GetPreferredMediaTypes(options ImageOptions) MediaTypes {
	if options.MediaTypes != MissingTypes {
		return options.MediaTypes
	}
	if options.MediaTypes == MissingTypes &&
		options.BaseImage == nil {
		return OCITypes
	}
	return DefaultTypes
}

type MediaTypes int

const (
	MissingTypes MediaTypes = iota
	DefaultTypes
	OCITypes
	DockerTypes
)

func (t MediaTypes) ManifestType() types.MediaType {
	switch t {
	case OCITypes:
		return types.OCIManifestSchema1
	case DockerTypes:
		return types.DockerManifestSchema2
	default:
		return ""
	}
}

func (t MediaTypes) ConfigType() types.MediaType {
	switch t {
	case OCITypes:
		return types.OCIConfigJSON
	case DockerTypes:
		return types.DockerConfigJSON
	default:
		return ""
	}
}

func (t MediaTypes) LayerType() types.MediaType {
	switch t {
	case OCITypes:
		return types.OCILayer
	case DockerTypes:
		return types.DockerLayer
	default:
		return ""
	}
}

func emptyV1(withPlatform Platform, withMediaTypes MediaTypes) (v1.Image, error) {
	configFile := &v1.ConfigFile{
		Architecture: withPlatform.Architecture,
		History:      []v1.History{},
		OS:           withPlatform.OS,
		OSVersion:    withPlatform.OSVersion,
		RootFS: v1.RootFS{
			Type:    "layers",
			DiffIDs: []v1.Hash{},
		},
	}
	image, err := mutate.ConfigFile(empty.Image, configFile)
	if err != nil {
		return nil, err
	}
	return EnsureMediaTypes(image, withMediaTypes)
}

func PreserveLayers(idx int, layer v1.Layer) (v1.Layer, error) {
	return layer, nil
}

// EnsureMediaTypes replaces the provided image with a new image that has the desired media types.
// It does this by constructing a manifest and config from the provided image,
// and adding the layers from the provided image to the new image with the right media type.
// If requested types are missing or default, it does nothing.
func EnsureMediaTypes(image v1.Image, requestedTypes MediaTypes) (v1.Image, error) {
	if requestedTypes == MissingTypes || requestedTypes == DefaultTypes {
		return image, nil
	}
	return EnsureMediaTypesAndLayers(image, requestedTypes, PreserveLayers)
}

// EnsureMediaTypesAndLayers replaces the provided image with a new image that has the desired media types.
// It does this by constructing a manifest and config from the provided image,
// and adding the layers from the provided image to the new image with the right media type.
// While adding the layers, each layer can be additionally mutated by providing a "mutate layer" function.
func EnsureMediaTypesAndLayers(image v1.Image, requestedTypes MediaTypes, mutateLayer func(idx int, layer v1.Layer) (v1.Layer, error)) (v1.Image, error) {
	// (1) get data from the original image
	// manifest
	beforeManifest, err := image.Manifest()
	if err != nil {
		return nil, err
	}
	// config
	beforeConfig, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}
	// layers
	beforeLayers, err := image.Layers()
	if err != nil {
		return nil, err
	}
	layersToSet := make([]v1.Layer, len(beforeLayers))
	for idx, layer := range beforeLayers {
		mutatedLayer, err := mutateLayer(idx, layer)
		if err != nil {
			return nil, err
		}
		layersToSet[idx] = mutatedLayer
	}

	// (2) construct a new image with the right manifest media type
	manifestType := requestedTypes.ManifestType()
	if manifestType == "" {
		manifestType = beforeManifest.MediaType
	}
	retImage := mutate.MediaType(empty.Image, manifestType)

	// (3) set config media type
	configType := requestedTypes.ConfigType()
	if configType == "" {
		configType = beforeManifest.Config.MediaType
	}
	// zero out history and diff IDs, as these will be updated when we call `mutate.Append` to add the layers
	beforeHistory := NormalizedHistory(beforeConfig.History, len(beforeConfig.RootFS.DiffIDs))
	beforeConfig.History = []v1.History{}
	beforeConfig.RootFS.DiffIDs = make([]v1.Hash, 0)
	// set config
	retImage, err = mutate.ConfigFile(retImage, beforeConfig)
	if err != nil {
		return nil, err
	}
	retImage = mutate.ConfigMediaType(retImage, configType)
	// (4) set layers with the right media type
	additions := layersAddendum(layersToSet, beforeHistory, requestedTypes.LayerType())
	if err != nil {
		return nil, err
	}
	retImage, err = mutate.Append(retImage, additions...)
	if err != nil {
		return nil, err
	}
	afterLayers, err := retImage.Layers()
	if err != nil {
		return nil, err
	}
	if len(afterLayers) != len(beforeLayers) {
		return nil, fmt.Errorf("found %d layers for image; expected %d", len(afterLayers), len(beforeLayers))
	}
	return retImage, nil
}

// layersAddendum creates an Addendum array with the given layers
// and the desired media type
func layersAddendum(layers []v1.Layer, history []v1.History, requestedType types.MediaType) []mutate.Addendum {
	addendums := make([]mutate.Addendum, 0)
	if len(history) != len(layers) {
		history = make([]v1.History, len(layers))
	}
	var err error
	for idx, layer := range layers {
		layerType := requestedType
		if requestedType == "" {
			// try to get a non-empty media type
			if layerType, err = layer.MediaType(); err != nil {
				layerType = ""
			}
		}
		addendums = append(addendums, mutate.Addendum{
			Layer:     layer,
			History:   history[idx],
			MediaType: layerType,
		})
	}
	return addendums
}

func NormalizedHistory(history []v1.History, nLayers int) []v1.History {
	if history == nil {
		return make([]v1.History, nLayers)
	}
	// ensure we remove history for empty layers
	var normalizedHistory []v1.History
	for _, h := range history {
		if !h.EmptyLayer {
			normalizedHistory = append(normalizedHistory, h)
		}
	}
	if len(normalizedHistory) == nLayers {
		return normalizedHistory
	}
	return make([]v1.History, nLayers)
}

func prepareNewWindowsImageIfNeeded(image *CNBImageCore) error {
	configFile, err := getConfigFile(image)
	if err != nil {
		return err
	}

	// only append base layer to empty image
	if !(configFile.OS == "windows") || len(configFile.RootFS.DiffIDs) > 0 {
		return nil
	}

	layerReader, err := layer.WindowsBaseLayer()
	if err != nil {
		return err
	}

	layerFile, err := os.CreateTemp("", "imgutil.local.image.windowsbaselayer")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer layerFile.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(layerFile, hasher)
	if _, err := io.Copy(multiWriter, layerReader); err != nil {
		return fmt.Errorf("copying base layer: %w", err)
	}

	diffID := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if err = image.AddLayerWithDiffIDAndHistory(layerFile.Name(), diffID, v1.History{}); err != nil {
		return fmt.Errorf("adding base layer to image: %w", err)
	}
	return nil
}
