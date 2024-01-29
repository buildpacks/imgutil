package imgutil

import (
	"fmt"
	"io"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

type Image interface {
	// getters

	Architecture() (string, error)
	CreatedAt() (time.Time, error)
	Entrypoint() ([]string, error)
	Env(key string) (string, error)
	// Found reports if image exists in the image store with `Name()`.
	Found() bool
	GetAnnotateRefName() (string, error)
	// GetLayer retrieves layer by diff id. Returns a reader of the uncompressed contents of the layer.
	GetLayer(diffID string) (io.ReadCloser, error)
	History() ([]v1.History, error)
	Identifier() (Identifier, error)
	// Kind exposes the type of image that backs the imgutil.Image implementation.
	// It could be `local`, `locallayout`, `remote`, or `layout`.
	Kind() string
	Label(string) (string, error)
	Labels() (map[string]string, error)
	// ManifestSize returns the size of the manifest. If a manifest doesn't exist, it returns 0.
	ManifestSize() (int64, error)
	Name() string
	OS() (string, error)
	OSVersion() (string, error)
	// TopLayer returns the diff id for the top layer
	TopLayer() (string, error)
	UnderlyingImage() v1.Image
	// Valid returns true if the image is well-formed (e.g. all manifest layers exist on the registry).
	Valid() bool
	Variant() (string, error)
	WorkingDir() (string, error)

	// setters

	// AnnotateRefName set a value for the `org.opencontainers.image.ref.name` annotation
	AnnotateRefName(refName string) error
	Rename(name string)
	SetArchitecture(string) error
	SetCmd(...string) error
	SetEntrypoint(...string) error
	SetEnv(string, string) error
	SetHistory([]v1.History) error
	SetLabel(string, string) error
	SetOS(string) error
	SetOSVersion(string) error
	SetVariant(string) error
	SetWorkingDir(string) error

	// modifiers

	AddLayer(path string) error
	AddLayerWithDiffID(path, diffID string) error
	AddLayerWithDiffIDAndHistory(path, diffID string, history v1.History) error
	Delete() error
	Rebase(string, Image) error
	RemoveLabel(string) error
	ReuseLayer(diffID string) error
	ReuseLayerWithHistory(diffID string, history v1.History) error
	// Save saves the image as `Name()` and any additional names provided to this method.
	Save(additionalNames ...string) error
	// SaveAs ignores the image `Name()` method and saves the image according to name & additional names provided to this method
	SaveAs(name string, additionalNames ...string) error
	// SaveFile saves the image as a docker archive and provides the filesystem location
	SaveFile() (string, error)
}

type Identifier fmt.Stringer

// Platform represents the target arch/os/os_version for an image construction and querying.
type Platform struct {
	Architecture string
	OS           string
	OSVersion    string
}

// OverrideHistoryIfNeeded zeroes out the history if the number of history entries doesn't match the number of layers.
func OverrideHistoryIfNeeded(image v1.Image) (v1.Image, error) {
	configFile, err := getConfigFile(image)
	if err != nil {
		return nil, err
	}
	configFile.History = NormalizedHistory(configFile.History, len(configFile.RootFS.DiffIDs))
	return mutate.ConfigFile(image, configFile)
}

type SaveDiagnostic struct {
	ImageName string
	Cause     error
}

type SaveError struct {
	Errors []SaveDiagnostic
}

func (e SaveError) Error() string {
	var errors []string
	for _, d := range e.Errors {
		errors = append(errors, fmt.Sprintf("[%s: %s]", d.ImageName, d.Cause.Error()))
	}
	return fmt.Sprintf("failed to write image to the following tags: %s", strings.Join(errors, ","))
}
