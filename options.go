package imgutil

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type ImageOptions struct {
	BaseImageRepoName     string
	PreviousImageRepoName string
	Config                *v1.Config
	CreatedAt             time.Time
	MediaTypes            MediaTypes
	Platform              Platform
	PreserveHistory       bool
	LayoutOptions
	RemoteOptions

	// These options must be specified in each implementation's image constructor
	BaseImage     v1.Image
	PreviousImage v1.Image
}

type LayoutOptions struct {
	PreserveDigest bool
	WithoutLayers  bool
}

type RemoteOptions struct {
	RegistrySettings    map[string]RegistrySetting
	AddEmptyLayerOnSave bool
}

type RegistrySetting struct {
	Insecure bool
}

type IndexAddOptions struct {
	All                          bool
	Local                        bool
	OS, Arch, Variant, OSVersion string
	Features, OSFeatures         []string
	Annotations                  map[string]string
	Image                        EditableImage
}

type IndexPushOptions struct {
	IndexFormatOptions
	IndexRemoteOptions
	Purge bool
	Tags  []string // Tags with which the index should be pushed to registry
}

type IndexFormatOptions struct {
	Format types.MediaType // The Format the Index should be. One of Docker or OCI
}

type IndexRemoteOptions struct {
	Insecure bool
}

type IndexOptions struct {
	IndexFormatOptions
	IndexRemoteOptions
	KeyChain          authn.Keychain
	XdgPath, Reponame string
}

func GetTransport(insecure bool) http.RoundTripper {
	if insecure {
		return &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // #nosec G402
			},
		}
	}

	return http.DefaultTransport
}
