package layout

import (
	"fmt"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/index"
)

// NewIndex will return a local OCI ImageIndex that can be modified and saved to a registry
func NewIndex(repoName string, ops ...index.Option) (idx imgutil.ImageIndex, err error) {
	var idxOps = &index.Options{}
	ops = append(ops, index.WithRepoName(repoName))

	for _, op := range ops {
		err = op(idxOps)
		if err != nil {
			return idx, err
		}
	}

	path, err := layout.FromPath(filepath.Join(idxOps.XDGRuntimePath(), imgutil.MakeFileSafeName(idxOps.RepoName())))
	if err != nil {
		return idx, err
	}

	imgIdx, err := path.ImageIndex()
	if err != nil {
		return idx, err
	}

	mfest, err := imgIdx.IndexManifest()
	if err != nil {
		return idx, err
	}

	if mfest == nil {
		return idx, imgutil.ErrManifestUndefined
	}

	if mfest.MediaType != types.OCIImageIndex {
		return nil, errors.New("no oci image index found")
	}

	idxOptions := imgutil.IndexOptions{
		KeyChain:         idxOps.Keychain(),
		XdgPath:          idxOps.XDGRuntimePath(),
		Reponame:         idxOps.RepoName(),
		InsecureRegistry: idxOps.Insecure(),
	}

	return imgutil.NewManifestHandler(imgIdx, idxOptions), nil
}

func NewImage(path string, ops ...ImageOption) (*Image, error) {
	options := &imgutil.ImageOptions{}
	for _, op := range ops {
		op(options)
	}

	options.Platform = processPlatformOption(options.Platform)

	var err error

	if options.BaseImage == nil && options.BaseImageRepoName != "" { // options.BaseImage supersedes options.BaseImageRepoName
		options.BaseImage, err = newImageFromPath(options.BaseImageRepoName, options.Platform)
		if err != nil {
			return nil, err
		}
	}
	options.MediaTypes = imgutil.GetPreferredMediaTypes(*options)
	if options.BaseImage != nil {
		options.BaseImage, err = newImageFacadeFrom(options.BaseImage, options.MediaTypes)
		if err != nil {
			return nil, err
		}
	}

	if options.PreviousImageRepoName != "" {
		options.PreviousImage, err = newImageFromPath(options.PreviousImageRepoName, options.Platform)
		if err != nil {
			return nil, err
		}
	}
	if options.PreviousImage != nil {
		options.PreviousImage, err = newImageFacadeFrom(options.PreviousImage, options.MediaTypes)
		if err != nil {
			return nil, err
		}
	}

	cnbImage, err := imgutil.NewCNBImage(*options)
	if err != nil {
		return nil, err
	}

	return &Image{
		CNBImageCore:      cnbImage,
		repoPath:          path,
		saveWithoutLayers: options.WithoutLayers,
		preserveDigest:    options.PreserveDigest,
	}, nil
}

func processPlatformOption(requestedPlatform imgutil.Platform) imgutil.Platform {
	var emptyPlatform imgutil.Platform
	if requestedPlatform != emptyPlatform {
		return requestedPlatform
	}
	return imgutil.Platform{
		OS:           "linux",
		Architecture: "amd64",
	}
}

// newImageFromPath creates a layout image from the given path.
// * If an image index for multiple platforms exists, it will try to select the image according to the platform provided.
// * If the image does not exist, then nothing is returned.
func newImageFromPath(path string, withPlatform imgutil.Platform) (v1.Image, error) {
	if !imageExists(path) {
		return nil, nil
	}

	layoutPath, err := FromPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load layout from path: %w", err)
	}
	index, err := layoutPath.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}
	image, err := imageFromIndex(index, withPlatform)
	if err != nil {
		return nil, fmt.Errorf("failed to load image from index: %w", err)
	}
	return image, nil
}

// imageFromIndex creates a v1.Image from the given Image Index, selecting the image manifest
// that matches the given OS and architecture.
func imageFromIndex(index v1.ImageIndex, platform imgutil.Platform) (v1.Image, error) {
	manifestList, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}
	if len(manifestList.Manifests) == 0 {
		return nil, fmt.Errorf("failed to find manifest at index")
	}

	// find manifest for platform
	var manifest v1.Descriptor
	if len(manifestList.Manifests) == 1 {
		manifest = manifestList.Manifests[0]
	} else {
		for _, m := range manifestList.Manifests {
			if m.Platform.OS == platform.OS &&
				m.Platform.Architecture == platform.Architecture {
				manifest = m
				break
			}
		}
		return nil, fmt.Errorf("failed to find manifest matching platform %v", platform)
	}

	return index.Image(manifest.Digest)
}
