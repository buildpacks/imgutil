package layout

import (
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

const ImageRefNameKey = "org.opencontainers.image.ref.name"

// ParseRefToPath parse the given image reference to local path directory following the rules:
// An image reference refers to either a tag reference or digest reference.
//   - A tag reference refers to an identifier of form <registry>/<repo>:<tag>
//   - A digest reference refers to a content addressable identifier of form <registry>/<repo>@<algorithm>:<digest>
//
// WHEN the image reference points to a tag reference returns <registry>/<repo>/<tag>
// WHEN the image reference points to a digest reference returns <registry>/<repo>/<algorithm>/<digest>
func ParseRefToPath(imageRef string) (string, error) {
	reference, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return "", err
	}
	path := filepath.Join(reference.Context().RegistryStr(), reference.Context().RepositoryStr())
	if strings.Contains(reference.Identifier(), ":") {
		splitDigest := strings.Split(reference.Identifier(), ":")
		path = filepath.Join(path, splitDigest[0], splitDigest[1])
	} else {
		path = filepath.Join(path, reference.Identifier())
	}

	return path, nil
}

// ImageRefAnnotation creates a map containing the key 'org.opencontainers.image.ref.name' with the provided value.
func ImageRefAnnotation(imageRefName string) map[string]string {
	if imageRefName == "" {
		return nil
	}
	annotations := make(map[string]string, 1)
	annotations[ImageRefNameKey] = imageRefName
	return annotations
}
