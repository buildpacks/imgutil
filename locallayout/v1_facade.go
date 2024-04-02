package locallayout

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	v1types "github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

// newV1ImageFacadeFromInspect returns a v1.Image constructed from the output of `docker inspect`.
// It is used to provide a v1.Image implementation for previous images and base images.
// The facade is never modified, but it may become the underlying v1.Image for imgutil.CNBImageCore images.
// The underlying layers will return data if they are contained in the store.
// By storing a pointer to the image store, callers can update the store to force the layers to return data.
func newV1ImageFacadeFromInspect(dockerInspect types.ImageInspect, history []image.HistoryResponseItem, withStore *Store, downloadLayersOnAccess bool) (v1.Image, error) {
	rootFS, err := toV1RootFS(dockerInspect.RootFS)
	if err != nil {
		return nil, err
	}
	configFile := &v1.ConfigFile{
		Architecture:  dockerInspect.Architecture,
		Author:        dockerInspect.Author,
		Container:     dockerInspect.Container,
		Created:       toV1Time(dockerInspect.Created),
		DockerVersion: dockerInspect.DockerVersion,
		History:       imgutil.NormalizedHistory(toV1History(history), len(dockerInspect.RootFS.Layers)),
		OS:            dockerInspect.Os,
		RootFS:        rootFS,
		Config:        toV1Config(dockerInspect.Config),
		OSVersion:     dockerInspect.OsVersion,
		Variant:       dockerInspect.Variant,
	}
	layersToSet := newEmptyLayerListFrom(configFile, dockerInspect.ID, withStore, downloadLayersOnAccess)
	return imageFrom(layersToSet, configFile, imgutil.DockerTypes)
}

func imageFrom(layers []v1.Layer, configFile *v1.ConfigFile, requestedTypes imgutil.MediaTypes) (v1.Image, error) {
	// (1) construct a new image with the right manifest media type
	manifestType := requestedTypes.ManifestType()
	retImage := mutate.MediaType(empty.Image, manifestType)

	// (2) set config media type
	configType := requestedTypes.ConfigType()
	// zero out history and diff IDs, as these will be updated when we call `mutate.Append` to add the layers
	beforeHistory := imgutil.NormalizedHistory(configFile.History, len(configFile.RootFS.DiffIDs))
	configFile.History = []v1.History{}
	configFile.RootFS.DiffIDs = make([]v1.Hash, 0)
	// set config
	var err error
	retImage, err = mutate.ConfigFile(retImage, configFile)
	if err != nil {
		return nil, err
	}
	retImage = mutate.ConfigMediaType(retImage, configType)
	// (3) set layers with the right media type
	additions := layersAddendum(layers, beforeHistory, requestedTypes.LayerType())
	retImage, err = mutate.Append(retImage, additions...)
	if err != nil {
		return nil, err
	}
	afterLayers, err := retImage.Layers()
	if err != nil {
		return nil, err
	}
	if len(afterLayers) != len(layers) {
		return nil, fmt.Errorf("found %d layers for image; expected %d", len(afterLayers), len(layers))
	}
	return retImage, nil
}

func layersAddendum(layers []v1.Layer, history []v1.History, requestedType v1types.MediaType) []mutate.Addendum {
	addendums := make([]mutate.Addendum, 0)
	if len(history) != len(layers) {
		history = make([]v1.History, len(layers))
	}
	for idx, layer := range layers {
		layerType := requestedType
		addendums = append(addendums, mutate.Addendum{
			Layer:     layer,
			History:   history[idx],
			MediaType: layerType,
		})
	}
	return addendums
}

func toV1RootFS(dockerRootFS types.RootFS) (v1.RootFS, error) {
	diffIDs := make([]v1.Hash, len(dockerRootFS.Layers))
	for idx, layer := range dockerRootFS.Layers {
		hash, err := v1.NewHash(layer)
		if err != nil {
			return v1.RootFS{}, err
		}
		diffIDs[idx] = hash
	}
	return v1.RootFS{
		Type:    dockerRootFS.Type,
		DiffIDs: diffIDs,
	}, nil
}

func toV1Time(dockerCreated string) v1.Time {
	createdAt, err := time.Parse(time.RFC3339Nano, dockerCreated)
	if err != nil {
		return v1.Time{Time: imgutil.NormalizedDateTime}
	}
	return v1.Time{Time: createdAt}
}

func toV1History(history []image.HistoryResponseItem) []v1.History {
	v1History := make([]v1.History, len(history))
	for offset, h := range history {
		// the daemon reports history in reverse order, so build up the array backwards
		v1History[len(v1History)-offset-1] = v1.History{
			Created:   v1.Time{Time: time.Unix(h.Created, 0)},
			CreatedBy: h.CreatedBy,
			Comment:   h.Comment,
		}
	}
	return v1History
}

func toV1Config(dockerCfg *container.Config) v1.Config {
	if dockerCfg == nil {
		return v1.Config{}
	}
	var healthcheck *v1.HealthConfig
	if dockerCfg.Healthcheck != nil {
		healthcheck = &v1.HealthConfig{
			Test:        dockerCfg.Healthcheck.Test,
			Interval:    dockerCfg.Healthcheck.Interval,
			Timeout:     dockerCfg.Healthcheck.Timeout,
			StartPeriod: dockerCfg.Healthcheck.StartPeriod,
			Retries:     dockerCfg.Healthcheck.Retries,
		}
	}
	exposedPorts := make(map[string]struct{}, len(dockerCfg.ExposedPorts))
	for key, val := range dockerCfg.ExposedPorts {
		exposedPorts[string(key)] = val
	}
	return v1.Config{
		AttachStderr:    dockerCfg.AttachStderr,
		AttachStdin:     dockerCfg.AttachStdin,
		AttachStdout:    dockerCfg.AttachStdout,
		Cmd:             dockerCfg.Cmd,
		Healthcheck:     healthcheck,
		Domainname:      dockerCfg.Domainname,
		Entrypoint:      dockerCfg.Entrypoint,
		Env:             dockerCfg.Env,
		Hostname:        dockerCfg.Hostname,
		Image:           dockerCfg.Image,
		Labels:          dockerCfg.Labels,
		OnBuild:         dockerCfg.OnBuild,
		OpenStdin:       dockerCfg.OpenStdin,
		StdinOnce:       dockerCfg.StdinOnce,
		Tty:             dockerCfg.Tty,
		User:            dockerCfg.User,
		Volumes:         dockerCfg.Volumes,
		WorkingDir:      dockerCfg.WorkingDir,
		ExposedPorts:    exposedPorts,
		ArgsEscaped:     dockerCfg.ArgsEscaped,
		NetworkDisabled: dockerCfg.NetworkDisabled,
		MacAddress:      dockerCfg.MacAddress,
		StopSignal:      dockerCfg.StopSignal,
		Shell:           dockerCfg.Shell,
	}
}

var _ v1.Layer = &v1LayerFacade{}

type v1LayerFacade struct {
	diffID v1.Hash
	store  *Store
	// for downloading layers from the daemon as needed
	downloadOnAccess bool
	imageIdentifier  string
}

func newEmptyLayerListFrom(configFile *v1.ConfigFile, withImageIdentifier string, withStore *Store, downloadOnAccess bool) []v1.Layer {
	layers := make([]v1.Layer, len(configFile.RootFS.DiffIDs))
	for idx, diffID := range configFile.RootFS.DiffIDs {
		layers[idx] = &v1LayerFacade{
			diffID:           diffID,
			store:            withStore,
			downloadOnAccess: downloadOnAccess,
			imageIdentifier:  withImageIdentifier,
		}
	}
	return layers
}

func (l v1LayerFacade) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (l v1LayerFacade) DiffID() (v1.Hash, error) {
	return l.diffID, nil
}

func (l v1LayerFacade) Digest() (v1.Hash, error) {
	return v1.Hash{}, nil
}

func (l v1LayerFacade) Uncompressed() (io.ReadCloser, error) {
	layer, err := l.store.LayerByDiffID(l.diffID)
	if err == nil {
		return layer.Uncompressed()
	}
	if !l.downloadOnAccess {
		return io.NopCloser(bytes.NewReader([]byte{})), nil
	}
	if err = l.store.downloadLayersFor(l.imageIdentifier); err != nil {
		return nil, err
	}
	layer, err = l.store.LayerByDiffID(l.diffID)
	if err != nil {
		return io.NopCloser(bytes.NewReader([]byte{})), nil
	}
	return layer.Uncompressed()
}

// Size returns a sentinel value indicating if the layer has data.
func (l v1LayerFacade) Size() (int64, error) {
	layer, err := l.store.LayerByDiffID(l.diffID)
	if err == nil {
		return layer.Size()
	}
	if !l.downloadOnAccess {
		return -1, nil
	}
	if err = l.store.downloadLayersFor(l.imageIdentifier); err != nil {
		return -1, err
	}
	layer, err = l.store.LayerByDiffID(l.diffID)
	if err != nil {
		return -1, nil
	}
	return layer.Size()
}

func (l v1LayerFacade) MediaType() (v1types.MediaType, error) {
	layer, err := l.store.LayerByDiffID(l.diffID)
	if err != nil {
		return v1types.OCILayer, nil
	}
	return layer.MediaType()
}
