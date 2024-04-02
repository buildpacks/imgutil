package locallayout

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil"
)

// NewImage returns a new image that can be modified and saved to a docker daemon
// via a tarball in legacy format.
func NewImage(repoName string, dockerClient DockerClient, ops ...func(*imgutil.ImageOptions)) (imgutil.Image, error) {
	options := &imgutil.ImageOptions{}
	for _, op := range ops {
		op(options)
	}

	var err error
	options.Platform, err = processDefaultPlatformOption(options.Platform, dockerClient)
	if err != nil {
		return nil, err
	}

	processPrevious, err := processImageOption(options.PreviousImageRepoName, dockerClient, true)
	if err != nil {
		return nil, err
	}
	if processPrevious.image != nil {
		options.PreviousImage = processPrevious.image
	}

	var (
		baseIdentifier string
		store          *Store
	)
	processBase, err := processImageOption(options.BaseImageRepoName, dockerClient, false)
	if err != nil {
		return nil, err
	}
	if processBase.image != nil {
		options.BaseImage = processBase.image
		baseIdentifier = processBase.identifier
		store = processBase.layerStore
	} else {
		store = &Store{dockerClient: dockerClient, downloadOnce: &sync.Once{}}
	}

	cnbImage, err := imgutil.NewCNBImage(*options)
	if err != nil {
		return nil, err
	}

	return &Image{
		CNBImageCore:   cnbImage,
		repoName:       repoName,
		store:          store,
		lastIdentifier: baseIdentifier,
		daemonOS:       options.Platform.OS,
	}, nil
}

func processDefaultPlatformOption(requestedPlatform v1.Platform, dockerClient DockerClient) (v1.Platform, error) {
	dockerPlatform, err := defaultPlatform(dockerClient)
	if err != nil {
		return v1.Platform{}, err
	}
	if dockerPlatform.Satisfies(requestedPlatform) {
		return dockerPlatform, nil
	}
	if requestedPlatform.OS != "" && requestedPlatform.OS != dockerPlatform.OS {
		return v1.Platform{},
			fmt.Errorf("invalid os: platform os %q must match the daemon os %q", requestedPlatform.OS, dockerPlatform.OS)
	}
	return requestedPlatform, nil
}

func defaultPlatform(dockerClient DockerClient) (v1.Platform, error) {
	daemonInfo, err := dockerClient.ServerVersion(context.Background())
	if err != nil {
		return v1.Platform{}, err
	}
	return v1.Platform{
		OS:           daemonInfo.Os,
		Architecture: daemonInfo.Arch,
		OSVersion:    daemonInfo.Version,
	}, nil
}

type imageResult struct {
	image      v1.Image
	identifier string
	layerStore *Store
}

func processImageOption(repoName string, dockerClient DockerClient, downloadLayersOnAccess bool) (imageResult, error) {
	if repoName == "" {
		return imageResult{}, nil
	}
	inspect, history, err := getInspectAndHistory(repoName, dockerClient)
	if err != nil {
		return imageResult{}, err
	}
	if inspect == nil {
		return imageResult{}, nil
	}
	layerStore := &Store{dockerClient: dockerClient, downloadOnce: &sync.Once{}}
	image, err := newV1ImageFacadeFromInspect(*inspect, history, layerStore, downloadLayersOnAccess)
	if err != nil {
		return imageResult{}, err
	}
	return imageResult{
		image:      image,
		identifier: inspect.ID,
		layerStore: layerStore,
	}, nil
}

func getInspectAndHistory(repoName string, dockerClient DockerClient) (*types.ImageInspect, []image.HistoryResponseItem, error) {
	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), repoName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("inspecting image %q: %w", repoName, err)
	}
	history, err := dockerClient.ImageHistory(context.Background(), repoName)
	if err != nil {
		return nil, nil, fmt.Errorf("get history for image %q: %w", repoName, err)
	}
	return &inspect, history, nil
}
