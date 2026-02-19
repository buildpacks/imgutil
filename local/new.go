package local

import (
	"cmp"
	"context"
	"fmt"
	"runtime"

	cerrdefs "github.com/containerd/errdefs"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
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
	var isPlatformAware bool
	options.Platform, isPlatformAware, err = processPlatformOption(options.Platform, dockerClient)
	if err != nil {
		return nil, err
	}

	previousImage, err := processImageOption(options.PreviousImageRepoName, isPlatformAware, options.Platform, dockerClient, true)
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
	baseImage, err := processImageOption(options.BaseImageRepoName, isPlatformAware, options.Platform, dockerClient, false)
	if err != nil {
		return nil, err
	}
	if baseImage.image != nil {
		options.BaseImage = baseImage.image
		baseIdentifier = baseImage.identifier
		store = baseImage.layerStore
	} else {
		if isPlatformAware {
			store = NewStoreWithPlatform(dockerClient, options.Platform)
		} else {
			store = NewStore(dockerClient)
		}
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

func defaultPlatform(dockerClient DockerClient) (imgutil.Platform, bool, error) {
	daemonInfo, err := dockerClient.ServerVersion(context.Background(), client.ServerVersionOptions{})
	if err != nil {
		return imgutil.Platform{}, false, err
	}
	isPlatformAware := versions.GreaterThanOrEqualTo(daemonInfo.APIVersion, "1.49")
	// When running on a different architecture than the daemon, we want to use images matching our own architecture
	// https://github.com/buildpacks/lifecycle/issues/1599
	if isPlatformAware {
		return imgutil.Platform{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
		}, isPlatformAware, nil
	}
	return imgutil.Platform{
		OS:           daemonInfo.Os,
		Architecture: daemonInfo.Arch,
	}, isPlatformAware, nil
}

func processPlatformOption(requestedPlatform imgutil.Platform, dockerClient DockerClient) (imgutil.Platform, bool, error) {
	dockerPlatform, isPlatformAware, err := defaultPlatform(dockerClient)
	if err != nil {
		return imgutil.Platform{}, false, err
	}
	if (requestedPlatform == imgutil.Platform{}) {
		return dockerPlatform, isPlatformAware, nil
	}
	return requestedPlatform, isPlatformAware, nil
}

type imageResult struct {
	image      v1.Image
	identifier string
	layerStore *Store
}

func processImageOption(repoName string, isPlatformAware bool, platform imgutil.Platform, dockerClient DockerClient, downloadLayersOnAccess bool) (imageResult, error) {
	if repoName == "" {
		return imageResult{}, nil
	}
	platformInspect, inspect, history, err := getInspectAndHistory(repoName, isPlatformAware, platform, dockerClient)
	if err != nil {
		return imageResult{}, err
	}
	// Use the platform-specific inspected value if possible, otherwise fall back to the generic inspect
	inspectForFacade := cmp.Or(platformInspect, inspect)
	if inspectForFacade == nil {
		return imageResult{}, nil
	}
	var layerStore *Store
	if isPlatformAware {
		layerStore = NewStoreWithPlatform(dockerClient, imgutil.Platform{
			Architecture: inspectForFacade.Architecture,
			OS:           inspectForFacade.Os,
			OSVersion:    inspectForFacade.OsVersion,
			Variant:      inspectForFacade.Variant,
		})
	} else {
		layerStore = NewStore(dockerClient)
	}
	v1Image, err := newV1ImageFacadeFromInspect(*inspectForFacade, history, layerStore, downloadLayersOnAccess)
	if err != nil {
		return imageResult{}, err
	}
	return imageResult{
		image:      v1Image,
		identifier: inspect.ID,
		layerStore: layerStore,
	}, nil
}

func getInspectAndHistory(repoName string, isPlatformAware bool, platform imgutil.Platform, dockerClient DockerClient) (*image.InspectResponse, *image.InspectResponse, []image.HistoryResponseItem, error) {
	inspect, err := dockerClient.ImageInspect(context.Background(), repoName)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("inspecting image %q: %w", repoName, err)
	}
	historyResult, err := dockerClient.ImageHistory(context.Background(), repoName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get history for image %q: %w", repoName, err)
	}

	if !isPlatformAware {
		return nil, &inspect.InspectResponse, historyResult.Items, nil
	}

	ociPlatform := ocispec.Platform{
		Architecture: platform.Architecture,
		OS:           platform.OS,
		OSVersion:    platform.OSVersion,
		Variant:      platform.Variant,
	}

	platformHistoryResult, err := dockerClient.ImageHistory(context.Background(), repoName, client.ImageHistoryWithPlatform(ociPlatform))
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, &inspect.InspectResponse, historyResult.Items, nil
		}
		return nil, nil, nil, fmt.Errorf("get history for image %q: %w", repoName, err)
	}

	platformInspect, err := dockerClient.ImageInspect(context.Background(), repoName, client.ImageInspectWithPlatform(&ociPlatform))
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, &inspect.InspectResponse, historyResult.Items, nil
		}
		return nil, nil, nil, fmt.Errorf("inspecting platform-specific image %q: %w", repoName, err)
	}

	return &platformInspect.InspectResponse, &inspect.InspectResponse, platformHistoryResult.Items, nil
}
