package remote

import (
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

const maxRetries = 2

type Image struct {
	*imgutil.CNBImageCore
	repoName            string
	keychain            authn.Keychain
	addEmptyLayerOnSave bool
	registrySettings    map[string]imgutil.RegistrySetting
}

func (i *Image) Kind() string {
	return `remote`
}

func (i *Image) Name() string {
	return i.repoName
}

func (i *Image) Rename(name string) {
	i.repoName = name
}

func (i *Image) Found() bool {
	_, err := i.found()
	return err == nil
}

func (i *Image) found() (*v1.Descriptor, error) {
	reg := getRegistrySetting(i.repoName, i.registrySettings)
	ref, auth, err := referenceForRepoName(i.keychain, i.repoName, reg.Insecure)
	if err != nil {
		return nil, err
	}
	return remote.Head(ref, remote.WithAuth(auth), remote.WithTransport(imgutil.GetTransport(reg.Insecure)))
}

func (i *Image) Identifier() (imgutil.Identifier, error) {
	ref, err := name.ParseReference(i.repoName, name.WeakValidation)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing reference for image %q", i.repoName)
	}

	hash, err := i.Digest()
	if err != nil {
		return nil, errors.Wrapf(err, "getting digest for image %q", i.repoName)
	}

	digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", ref.Context().Name(), hash.String()), name.WeakValidation)
	if err != nil {
		return nil, errors.Wrap(err, "creating digest reference")
	}

	return DigestIdentifier{
		Digest: digestRef,
	}, nil
}

// Valid returns true if the (saved) image is valid.
func (i *Image) Valid() bool {
	return i.valid() == nil
}

func (i *Image) valid() error {
	reg := getRegistrySetting(i.repoName, i.registrySettings)
	ref, auth, err := referenceForRepoName(i.keychain, i.repoName, reg.Insecure)
	if err != nil {
		return err
	}
	desc, err := remote.Get(ref, remote.WithAuth(auth), remote.WithTransport(imgutil.GetTransport(reg.Insecure)))
	if err != nil {
		return err
	}
	if desc.MediaType == types.OCIImageIndex || desc.MediaType == types.DockerManifestList {
		index, err := desc.ImageIndex()
		if err != nil {
			return err
		}
		return validate.Index(index, validate.Fast)
	}
	img, err := desc.Image()
	if err != nil {
		return err
	}
	return validate.Image(img, validate.Fast)
}

func (i *Image) Delete() error {
	id, err := i.Identifier()
	if err != nil {
		return err
	}
	reg := getRegistrySetting(i.repoName, i.registrySettings)
	ref, auth, err := referenceForRepoName(i.keychain, id.String(), reg.Insecure)
	if err != nil {
		return err
	}
	return remote.Delete(ref, remote.WithAuth(auth), remote.WithTransport(imgutil.GetTransport(reg.Insecure)))
}

// extras

func (i *Image) CheckReadAccess() (bool, error) {
	var err error
	if _, err = i.found(); err == nil {
		return true, nil
	}
	var canRead bool
	if transportErr, ok := err.(*transport.Error); ok {
		if canRead = transportErr.StatusCode != http.StatusUnauthorized &&
			transportErr.StatusCode != http.StatusForbidden; canRead {
			err = nil
		}
	}
	return canRead, err
}

func (i *Image) CheckReadWriteAccess() (bool, error) {
	if canRead, err := i.CheckReadAccess(); !canRead {
		return false, err
	}
	reg := getRegistrySetting(i.repoName, i.registrySettings)
	ref, _, err := referenceForRepoName(i.keychain, i.repoName, reg.Insecure)
	if err != nil {
		return false, err
	}
	err = remote.CheckPushPermission(ref, i.keychain, imgutil.GetTransport(reg.Insecure))
	if err != nil {
		return false, err
	}
	return true, nil
}

// platformsMatch checks if two platforms match according to the following rules:
// - OS and Architecture must match exactly
// - Variant and OSVersion are optional - if either is blank in either platform, it's considered a match
func platformsMatch(p1, p2 *imgutil.Platform) bool {
	if p1 == nil || p2 == nil {
		return false
	}

	// OS and Architecture must match exactly
	if p1.OS != p2.OS || p1.Architecture != p2.Architecture {
		return false
	}

	// For Variant and OSVersion, if either is blank, consider it a match
	variantMatch := p1.Variant == "" || p2.Variant == "" || p1.Variant == p2.Variant
	osVersionMatch := p1.OSVersion == "" || p2.OSVersion == "" || p1.OSVersion == p2.OSVersion

	return variantMatch && osVersionMatch
}

// platformString returns a pretty-printed string representation of a platform's variant and OS version.
// Returns empty string if both are blank, otherwise returns "/variant:osversion" format.
func platformString(platform *imgutil.Platform) string {
	if platform == nil {
		return ""
	}

	var parts []string

	if platform.Variant != "" {
		parts = append(parts, platform.Variant)
	}

	if platform.OSVersion != "" {
		parts = append(parts, platform.OSVersion)
	}

	if len(parts) == 0 {
		return ""
	}

	result := "/" + parts[0]
	if len(parts) > 1 {
		result += ":" + parts[1]
	}

	return result
}

// ResolvePlatformSpecificDigest resolves a manifest list digest to a platform-specific digest.
// Accepts both digest references (repo@sha256:abc123...) and tag references (repo:tag or repo).
// If platform is nil, returns the provided reference unchanged.
// If the provided reference points to a single manifest (not a manifest list),
// it validates that the platform matches and returns the same reference.
// If it points to a manifest list, it finds and returns the digest reference for the manifest that matches the specified platform.
func ResolvePlatformSpecificDigest(imageRef string, platform *imgutil.Platform, keychain authn.Keychain, registrySettings map[string]imgutil.RegistrySetting) (string, error) {
	// If platform is nil, return the reference unchanged
	if platform == nil {
		return imageRef, nil
	}

	// Parse the reference (could be digest or tag)
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return "", errors.Wrapf(err, "parsing image reference %q", imageRef)
	}

	// Get registry settings for the reference
	reg := getRegistrySetting(imageRef, registrySettings)

	// Get authentication
	auth, err := keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return "", errors.Wrapf(err, "resolving authentication for registry %q", ref.Context().Registry)
	}

	// Fetch the descriptor
	desc, err := remote.Get(ref, remote.WithAuth(auth), remote.WithTransport(imgutil.GetTransport(reg.Insecure)))
	if err != nil {
		return "", errors.Wrapf(err, "fetching descriptor for %q", imageRef)
	}

	// Check if it's a manifest list
	if desc.MediaType == types.OCIImageIndex || desc.MediaType == types.DockerManifestList {
		// Get the index
		index, err := desc.ImageIndex()
		if err != nil {
			return "", errors.Wrapf(err, "getting image index for %q", imageRef)
		}

		// Get the manifest list
		manifestList, err := index.IndexManifest()
		if err != nil {
			return "", errors.Wrapf(err, "getting manifest list for %q", imageRef)
		}

		// Find the platform-specific manifest
		for _, manifest := range manifestList.Manifests {
			if manifest.Platform != nil {
				manifestPlatform := &imgutil.Platform{
					OS:           manifest.Platform.OS,
					Architecture: manifest.Platform.Architecture,
					Variant:      manifest.Platform.Variant,
					OSVersion:    manifest.Platform.OSVersion,
				}

				if platformsMatch(platform, manifestPlatform) {
					// Create a new digest reference for the platform-specific manifest
					platformDigestRef, err := name.NewDigest(
						fmt.Sprintf("%s@%s", ref.Context().Name(), manifest.Digest.String()),
						name.WeakValidation,
					)
					if err != nil {
						return "", errors.Wrapf(err, "creating platform-specific digest reference")
					}
					return platformDigestRef.String(), nil
				}
			}
		}

		return "", errors.Errorf("no manifest found for platform %s/%s%s in manifest list %q",
			platform.OS,
			platform.Architecture,
			platformString(platform),
			imageRef)
	}

	// If it's a single manifest, validate that the platform matches
	img, err := desc.Image()
	if err != nil {
		return "", errors.Wrapf(err, "getting image for %q", imageRef)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return "", errors.Wrapf(err, "getting config file for %q", imageRef)
	}

	// Create platform from image config
	imagePlatform := &imgutil.Platform{
		OS:           configFile.OS,
		Architecture: configFile.Architecture,
		Variant:      configFile.Variant,
		OSVersion:    configFile.OSVersion,
	}

	// Check if the image's platform matches the requested platform
	if !platformsMatch(platform, imagePlatform) {
		return "", errors.Errorf("image platform %s/%s%s does not match requested platform %s/%s%s for %q",
			configFile.OS,
			configFile.Architecture,
			platformString(imagePlatform),
			platform.OS,
			platform.Architecture,
			platformString(platform),
			imageRef)
	}

	// Platform matches - if input was a digest reference, return it unchanged
	// If input was a tag reference, return the digest reference for consistency
	if _, ok := ref.(name.Digest); ok {
		return imageRef, nil
	}

	// Convert tag reference to digest reference
	digest, err := img.Digest()
	if err != nil {
		return "", errors.Wrapf(err, "getting digest for image %q", imageRef)
	}

	digestRef, err := name.NewDigest(
		fmt.Sprintf("%s@%s", ref.Context().Name(), digest.String()),
		name.WeakValidation,
	)
	if err != nil {
		return "", errors.Wrapf(err, "creating digest reference for %q", imageRef)
	}

	return digestRef.String(), nil
}

var _ imgutil.ImageIndex = (*ImageIndex)(nil)

type ImageIndex struct {
	*imgutil.CNBIndex
}
