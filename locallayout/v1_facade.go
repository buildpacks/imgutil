package locallayout

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
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

// v1ImageFacade wraps a v1.Image constructed from the output of `docker inspect`.
// It is used to provide a v1.Image implementation for previous images and base images.
// The v1ImageFacade is never modified, but it may become the underlying v1.Image for imgutil.CNBImageCore images.
// A v1ImageFacade will try to return layer data if the layers exist on disk,
// otherwise it will return empty layer data.
// By storing a pointer to the image store, users can update the store to force a v1ImageFacade to return layer data.
type v1ImageFacade struct {
	v1.Image
	emptyLayers []v1.Layer

	// for downloading layers from the daemon as needed
	store                  imgutil.ImageStore
	downloadLayersOnAccess bool // set to true to downloading ALL the image layers from the daemon when LayerByDiffID is called
	downloadOnce           *sync.Once
	identifier             string
}

var _ v1.Image = &v1ImageFacade{}

func (i *v1ImageFacade) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	if layer := findLayer(h, i.store.Layers()); layer != nil {
		return layer, nil
	}
	if i.downloadLayersOnAccess {
		if err := i.ensureLayers(); err != nil {
			return nil, err
		}
		if layer := findLayer(h, i.store.Layers()); layer != nil {
			return layer, nil
		}
	}
	if layer := findLayer(h, i.emptyLayers); layer != nil {
		return layer, nil
	}
	return nil, fmt.Errorf("failed to find layer with diff ID %q", h.String())
}

func (i *v1ImageFacade) ensureLayers() error {
	var err error
	i.downloadOnce.Do(func() {
		err = i.store.DownloadLayersFor(i.identifier)
	})
	if err != nil {
		return fmt.Errorf("fetching base layers: %w", err)
	}
	return nil
}

func findLayer(withHash v1.Hash, inLayers []v1.Layer) v1.Layer {
	for _, layer := range inLayers {
		layerHash, err := layer.DiffID()
		if err != nil {
			continue
		}
		if layerHash.String() == withHash.String() {
			return layer
		}
	}
	return nil
}

func newV1ImageFacadeFromInspect(dockerInspect types.ImageInspect, history []image.HistoryResponseItem, dockerClient DockerClient, downloadLayersOnAccess bool) (*v1ImageFacade, error) {
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
	img, err := mutate.ConfigFile(empty.Image, configFile)
	if err != nil {
		return nil, err
	}
	return &v1ImageFacade{
		Image:                  img,
		emptyLayers:            newEmptyLayerListFrom(configFile),
		store:                  &Store{dockerClient: dockerClient},
		downloadLayersOnAccess: downloadLayersOnAccess,
		downloadOnce:           &sync.Once{},
		identifier:             dockerInspect.ID,
	}, nil
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
	diffID            v1.Hash
	optionalLayerPath string
}

func newEmptyLayerListFrom(configFile *v1.ConfigFile) []v1.Layer {
	layers := make([]v1.Layer, len(configFile.RootFS.DiffIDs))
	for idx, diffID := range configFile.RootFS.DiffIDs {
		layers[idx] = &v1LayerFacade{
			diffID: diffID,
		}
	}
	return layers
}

func (l v1LayerFacade) Digest() (v1.Hash, error) {
	return l.diffID, nil
}

func (l v1LayerFacade) DiffID() (v1.Hash, error) {
	return l.diffID, nil
}

func (l v1LayerFacade) Compressed() (io.ReadCloser, error) {
	if l.optionalLayerPath != "" {
		f, err := os.Open(l.optionalLayerPath)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

func (l v1LayerFacade) Uncompressed() (io.ReadCloser, error) {
	if l.optionalLayerPath != "" {
		f, err := os.Open(l.optionalLayerPath)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return io.NopCloser(bytes.NewReader([]byte{})), nil
}

// Size returns a sentinel value indicating if the layer has data.
func (l v1LayerFacade) Size() (int64, error) {
	if l.optionalLayerPath != "" {
		return 1, nil
	}
	return -1, nil
}

func (l v1LayerFacade) MediaType() (v1types.MediaType, error) {
	return v1types.OCILayer, nil
}
