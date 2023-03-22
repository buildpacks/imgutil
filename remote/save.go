package remote

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

func (i *Image) Save(additionalNames ...string) error {
	return i.SaveAs(i.Name(), additionalNames...)
}

func (i *Image) SaveAs(name string, additionalNames ...string) error {
	var err error

	allNames := append([]string{name}, additionalNames...)

	i.image, err = mutate.CreatedAt(i.image, v1.Time{Time: i.createdAt})
	if err != nil {
		return errors.Wrap(err, "set creation time")
	}

	cfg, err := i.image.ConfigFile()
	if err != nil {
		return errors.Wrap(err, "get image config")
	}
	cfg = cfg.DeepCopy()

	layers, err := i.image.Layers()
	if err != nil {
		return errors.Wrap(err, "get image layers")
	}
	cfg.History = make([]v1.History, len(layers))
	for j := range cfg.History {
		cfg.History[j] = v1.History{
			Created: v1.Time{Time: i.createdAt},
		}
	}

	cfg.DockerVersion = ""
	cfg.Container = ""
	i.image, err = mutate.ConfigFile(i.image, cfg)
	if err != nil {
		return errors.Wrap(err, "zeroing history")
	}

	if len(layers) == 0 && i.addEmptyLayerOnSave {
		empty := static.NewLayer([]byte{}, types.OCILayer)
		i.image, err = mutate.AppendLayers(i.image, empty)
		if err != nil {
			return errors.Wrap(err, "empty layer could not be added")
		}
	}

	var diagnostics []imgutil.SaveDiagnostic
	for _, n := range allNames {
		if err := i.doSave(n); err != nil {
			diagnostics = append(diagnostics, imgutil.SaveDiagnostic{ImageName: n, Cause: err})
		}
	}
	if len(diagnostics) > 0 {
		return imgutil.SaveError{Errors: diagnostics}
	}

	return nil
}

func (i *Image) doSave(imageName string) error {
	reg := getRegistry(i.repoName, i.registrySettings)
	ref, auth, err := referenceForRepoName(i.keychain, imageName, reg.insecure)
	if err != nil {
		return err
	}
	return remote.Write(ref, i.image, remote.WithAuth(auth))
}
