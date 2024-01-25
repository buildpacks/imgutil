package locallayout

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/buildpacks/imgutil"
)

// NewImage returns a new image that can be modified and saved to a docker daemon
// via a tarball in:
// * OCI layout format if the daemon uses containerd as the image store
// * legacy format if the daemon uses graph drivers.
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

	options.PreviousImage, err = processPreviousImageOption(options.PreviousImageRepoName, dockerClient)
	if err != nil {
		return nil, err
	}

	store := &Store{dockerClient: dockerClient}
	baseImage, err := processBaseImageOption(options.BaseImageRepoName, dockerClient, store)
	if err != nil {
		return nil, err
	}
	var baseIdentifier string
	if baseImage != nil {
		options.BaseImage = baseImage
		baseIdentifier = baseImage.identifier
	}

	cnbImage, err := imgutil.NewCNBImage(repoName, *options)
	if err != nil {
		return nil, err
	}

	return &Image{
		CNBImageCore:   cnbImage,
		store:          store,
		lastIdentifier: baseIdentifier,
		daemonOS:       options.Platform.OS,
		downloadOnce:   &sync.Once{},
	}, nil
}

func processDefaultPlatformOption(requestedPlatform imgutil.Platform, dockerClient DockerClient) (imgutil.Platform, error) {
	dockerPlatform, err := defaultPlatform(dockerClient)
	if err != nil {
		return imgutil.Platform{}, err
	}
	if (requestedPlatform == imgutil.Platform{}) {
		return dockerPlatform, nil
	}
	if requestedPlatform.OS != "" && requestedPlatform.OS != dockerPlatform.OS {
		return imgutil.Platform{},
			fmt.Errorf("invalid os: platform os %q must match the daemon os %q", requestedPlatform.OS, dockerPlatform.OS)
	}
	return requestedPlatform, nil
}

func defaultPlatform(dockerClient DockerClient) (imgutil.Platform, error) {
	daemonInfo, err := dockerClient.ServerVersion(context.Background())
	if err != nil {
		return imgutil.Platform{}, err
	}
	return imgutil.Platform{
		OS:           daemonInfo.Os,
		Architecture: daemonInfo.Arch,
	}, nil
}

func processPreviousImageOption(repoName string, dockerClient DockerClient) (*v1ImageFacade, error) {
	if repoName == "" {
		return nil, nil
	}
	inspect, history, err := getInspectAndHistory(repoName, dockerClient)
	if err != nil {
		return nil, err
	}
	if inspect == nil {
		return nil, nil
	}
	return newV1ImageFacadeFromInspect(*inspect, history, &Store{dockerClient: dockerClient}, true)
}

func processBaseImageOption(repoName string, dockerClient DockerClient, store *Store) (*v1ImageFacade, error) {
	if repoName == "" {
		return nil, nil
	}
	inspect, history, err := getInspectAndHistory(repoName, dockerClient)
	if err != nil {
		return nil, err
	}
	if inspect == nil {
		return nil, nil
	}
	var downloadLayersOnAccess bool
	if usesContainerdStorage(dockerClient) {
		downloadLayersOnAccess = true
	}
	return newV1ImageFacadeFromInspect(*inspect, history, store, downloadLayersOnAccess)
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
