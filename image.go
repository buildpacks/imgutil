package imgutil

import (
	"io"
	"time"
)

type SaveResult struct {
	// Digest is the digest of the image
	Digest string
	// Outcomes is a map of image name to `error` or `nil` if saved properly.
	Outcomes map[string]error
}

func NewFailedResult(imageNames []string, err error) SaveResult {
	errs := map[string]error{}
	for _, n := range imageNames {
		errs[n] = err
	}

	return SaveResult{
		Outcomes: errs,
	}
}

type Image interface {
	Name() string
	Rename(name string)
	Digest() (string, error)
	Label(string) (string, error)
	SetLabel(string, string) error
	Env(key string) (string, error)
	SetEnv(string, string) error
	SetEntrypoint(...string) error
	SetWorkingDir(string) error
	SetCmd(...string) error
	Rebase(string, Image) error
	AddLayer(path string) error
	ReuseLayer(sha string) error
	TopLayer() (string, error)
	// Save saves the image as `Name()` and any additional names provided to this method.
	Save(additionalNames ...string) SaveResult
	// Found tells whether the image exists in the repository by `Name()`.
	Found() bool
	GetLayer(sha string) (io.ReadCloser, error)
	Delete() error
	CreatedAt() (time.Time, error)
}
