package imgutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"github.com/buildpacks/imgutil/layer"
)

func NewCNBImage(repoName string, store ImageStore, options ImageOptions) (*CNBImageCore, error) {
	image := &CNBImageCore{
		Image: options.BaseImage,
		Store: store,
		// required
		repoName: repoName,
		// optional
		preferredMediaTypes: options.MediaTypes,
		preserveHistory:     options.PreserveHistory,
		previousImage:       options.PreviousImage,
	}

	var err error
	if image.Image == nil {
		image.Image, err = emptyV1(options.Platform)
		if err != nil {
			return nil, err
		}
	}
	if image.Image, err = OverrideMediaTypes(image.Image, options.MediaTypes); err != nil {
		return nil, err
	}

	if err = prepareNewWindowsImageIfNeeded(image); err != nil {
		return nil, err
	}

	createdAt := NormalizedDateTime
	if !options.CreatedAt.IsZero() {
		createdAt = options.CreatedAt
	}
	if err = image.MutateConfigFile(func(c *v1.ConfigFile) {
		c.Created = v1.Time{Time: createdAt}
		c.Container = ""
	}); err != nil {
		return nil, err
	}

	if !options.PreserveHistory {
		if err = image.MutateConfigFile(func(c *v1.ConfigFile) {
			for j := range c.History {
				c.History[j] = v1.History{Created: v1.Time{Time: createdAt}}
			}
		}); err != nil {
			return nil, err
		}
	}

	if options.Config != nil {
		if err = image.MutateConfigFile(func(c *v1.ConfigFile) {
			c.Config = *options.Config
		}); err != nil {
			return nil, err
		}
	}

	return image, nil
}

func emptyV1(withPlatform Platform) (v1.Image, error) {
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
	return mutate.ConfigFile(empty.Image, configFile)
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
