package local

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/buildpacks/imgutil"
)

// NewImage returns a new image that can be modified and saved to a docker daemon
// via a tarball in legacy format.
func NewImage(repoName string, dockerClient DockerClient, ops ...imgutil.ImageOption) (*Image, error) {
	options := &imgutil.ImageOptions{}
	for _, op := range ops {
		op(options)
	}

	var err error
	options.Platform, err = processPlatformOption(options.Platform, dockerClient)
	if err != nil {
		return nil, err
	}

	previousImage, err := processImageOption(options.PreviousImageRepoName, options.Platform, dockerClient, true)
	if err != nil {
		return nil, err
	}
	if previousImage.image != nil {
		options.PreviousImage = previousImage.image
	}

	var (
		baseIdentifier string
		store          *Store
	)
	baseImage, err := processImageOption(options.BaseImageRepoName, options.Platform, dockerClient, false)
	if err != nil {
		return nil, err
	}
	if baseImage.image != nil {
		options.BaseImage = baseImage.image
		baseIdentifier = baseImage.identifier
		store = baseImage.layerStore
	} else {
		store = NewStore(dockerClient)
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

func defaultPlatform(dockerClient DockerClient) (imgutil.Platform, error) {
	daemonInfo, err := dockerClient.ServerVersion(context.Background(), client.ServerVersionOptions{})
	if err != nil {
		return imgutil.Platform{}, err
	}
	if daemonInfo.Os == "linux" {
		// When running on a different architecture than the daemon, we still want to use images matching our own architecture
		// https://github.com/buildpacks/lifecycle/issues/1599
		return imgutil.Platform{
			OS:           "linux",
			Architecture: runtime.GOARCH,
		}, nil
	}
	return imgutil.Platform{
		OS:           daemonInfo.Os,
		Architecture: daemonInfo.Arch,
	}, nil
}

func processPlatformOption(requestedPlatform imgutil.Platform, dockerClient DockerClient) (imgutil.Platform, error) {
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

type imageResult struct {
	image      v1.Image
	identifier string
	layerStore *Store
}

func processImageOption(repoName string, platform imgutil.Platform, dockerClient DockerClient, downloadLayersOnAccess bool) (imageResult, error) {
	if repoName == "" {
		return imageResult{}, nil
	}
	inspect, history, err := getInspectAndHistory(repoName, platform, dockerClient)
	if err != nil {
		return imageResult{}, err
	}
	if inspect == nil {
		return imageResult{}, nil
	}
	layerStore := NewStore(dockerClient)
	v1Image, err := newV1ImageFacadeFromInspect(*inspect, history, layerStore, downloadLayersOnAccess)
	if err != nil {
		return imageResult{}, err
	}
	return imageResult{
		image:      v1Image,
		identifier: inspect.ID,
		layerStore: layerStore,
	}, nil
}

func getInspectAndHistory(repoName string, platform imgutil.Platform, dockerClient DockerClient) (*image.InspectResponse, []image.HistoryResponseItem, error) {
	platformOpt := client.ImageInspectWithPlatform(&ocispec.Platform{
		Architecture: platform.Architecture,
		OS:           platform.OS,
		OSVersion:    platform.OSVersion,
		Variant:      platform.Variant,
	})
	// Try to inspect the image with the default platform/arch
	inspect, err := dockerClient.ImageInspect(context.Background(), repoName, platformOpt)
	if err != nil {
		// ...and if that fails, inspect without the platform
		if cerrdefs.IsNotImplemented(err) || strings.Contains(err.Error(), "requires API version") {
			fmt.Printf("Docker API Version < 1.49. Platform defaulting to daemon platform\n")
			inspect, err = dockerClient.ImageInspect(context.Background(), repoName)
		} else if cerrdefs.IsNotFound(err) {
			fmt.Printf("Docker did not find image %s with platform %s/%s; retrying without specifying a platform\n", repoName, platform.OS, platform.Architecture)
			inspect, err = dockerClient.ImageInspect(context.Background(), repoName)
		}
	}
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("inspecting image %q: %w", repoName, err)
	}
	historyResult, err := dockerClient.ImageHistory(context.Background(), repoName)
	if err != nil {
		return nil, nil, fmt.Errorf("get history for image %q: %w", repoName, err)
	}
	return &inspect.InspectResponse, historyResult.Items, nil
}
