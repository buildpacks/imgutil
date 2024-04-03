package locallayout

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil"
)

// Image wraps an imgutil.CNBImageCore and implements the methods needed to complete the imgutil.Image interface.
type Image struct {
	*imgutil.CNBImageCore
	repoName       string
	store          *Store
	lastIdentifier string
	daemonOS       string
	mutex          sync.Mutex
}

func (i *Image) Kind() string {
	return "locallayout"
}

func (i *Image) Name() string {
	return i.repoName
}

func (i *Image) Rename(name string) {
	i.repoName = name
}

func (i *Image) Found() bool {
	return i.lastIdentifier != ""
}

func (i *Image) Identifier() (imgutil.Identifier, error) {
	return idStringer{
		id: strings.TrimPrefix(i.lastIdentifier, "sha256:"),
	}, nil
}

type idStringer struct {
	id string
}

func (i idStringer) String() string {
	return i.id
}

// GetLayer returns an io.ReadCloser with uncompressed layer data.
// The layer will always have data, even if that means downloading ALL the image layers from the daemon.
func (i *Image) GetLayer(diffID string) (io.ReadCloser, error) {
	layerHash, err := v1.NewHash(diffID)
	if err != nil {
		return nil, err
	}
	layer, err := i.LayerByDiffID(layerHash)
	if err == nil {
		// this avoids downloading ALL the image layers from the daemon
		// if the layer is available locally
		// (e.g., it was added using AddLayer).
		if size, err := layer.Size(); err != nil && size != -1 {
			return layer.Uncompressed()
		}
	}
	if err = i.ensureLayers(); err != nil {
		return nil, err
	}
	layer, err = i.LayerByDiffID(layerHash)
	if err != nil {
		return nil, fmt.Errorf("image %q does not contain layer with diff ID %q", i.Name(), layerHash.String())
	}
	return layer.Uncompressed()
}

func (i *Image) ensureLayers() error {
	if err := i.store.downloadLayersFor(i.lastIdentifier); err != nil {
		return fmt.Errorf("fetching base layers: %w", err)
	}
	return nil
}

func (i *Image) SetOS(osVal string) error {
	if osVal != i.daemonOS {
		return errors.New("invalid os: must match the daemon")
	}
	return i.CNBImageCore.SetOS(osVal)
}

func (i *Image) Rebase(baseTopLayerDiffID string, withNewBase imgutil.Image) error {
	if err := i.ensureLayers(); err != nil {
		return err
	}
	return i.CNBImageCore.Rebase(baseTopLayerDiffID, withNewBase)
}

func (i *Image) Save(additionalNames ...string) error {
	err := i.SetCreatedAtAndHistory()
	if err != nil {
		return err
	}
	i.lastIdentifier, err = i.store.Save(i, i.Name(), additionalNames...)
	return err
}

func (i *Image) SaveAs(name string, additionalNames ...string) error {
	err := i.SetCreatedAtAndHistory()
	if err != nil {
		return err
	}
	i.lastIdentifier, err = i.store.Save(i, name, additionalNames...)
	return err
}

func (i *Image) SaveFile() (string, error) {
	return i.store.SaveFile(i, i.Name())
}

func (i *Image) Delete() error {
	return i.store.Delete(i.lastIdentifier)
}
