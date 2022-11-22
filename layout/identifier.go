package layout

import (
	"crypto/sha256"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type IDIdentifier struct {
	ImageID string
}

func newIDIdentifier(configFile *v1.ConfigFile) (IDIdentifier, error) {
	return IDIdentifier{
		ImageID: "sha256:" + asSha256(configFile),
	}, nil
}

func (i IDIdentifier) String() string {
	return i.ImageID
}

func asSha256(o interface{}) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", o)))
	return fmt.Sprintf("%x", h.Sum(nil))
}
