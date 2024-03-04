package remote

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

func (i *Image) Save(additionalNames ...string) error {
	return i.SaveAs(i.Name(), additionalNames...)
}

func (i *Image) SaveAs(name string, additionalNames ...string) error {
	var err error
	allNames := append([]string{name}, additionalNames...)

	// create time
	if i.image, err = mutate.CreatedAt(i.image, v1.Time{Time: i.createdAt}); err != nil {
		return fmt.Errorf("setting creation time: %w", err)
	}

	// history
	if i.image, err = imgutil.OverrideHistoryIfNeeded(i.image); err != nil {
		return fmt.Errorf("overriding history: %w", err)
	}
	cfg, err := i.image.ConfigFile()
	if err != nil {
		return fmt.Errorf("getting config file: %w", err)
	}
	created := v1.Time{Time: i.createdAt}
	if i.withHistory {
		// set created
		for j := range cfg.History {
			cfg.History[j].Created = created
		}
	} else {
		// zero history, set created
		for j := range cfg.History {
			cfg.History[j] = v1.History{Created: created}
		}
	}

	// docker, container
	cfg.DockerVersion = ""
	cfg.Container = ""

	// commit config
	i.image, err = mutate.ConfigFile(i.image, cfg)
	if err != nil {
		return fmt.Errorf("zeroing history: %w", err)
	}

	// layers
	layers, err := i.image.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", err)
	}
	if len(layers) == 0 && i.addEmptyLayerOnSave {
		empty := static.NewLayer([]byte{}, types.OCILayer)
		i.image, err = mutate.AppendLayers(i.image, empty)
		if err != nil {
			return fmt.Errorf("adding empty layer: %w", err)
		}
	}

	i.image, err = imgutil.MutateManifest(i.image, func(mfest *v1.Manifest) {
		config := mfest.Config
		if len(i.annotations) != 0 {
			mfest.Annotations = i.annotations
			config.Annotations = i.annotations
		}

		if len(i.urls) != 0 {
			config.URLs = append(config.URLs, i.urls...)
		}

		if config.Platform == nil {
			config.Platform = &v1.Platform{}
		}

		if len(i.features) != 0 {
			config.Platform.Features = append(config.Platform.Features, i.features...)
		}

		if len(i.osFeatures) != 0 {
			config.Platform.OSFeatures = append(config.Platform.OSFeatures, i.osFeatures...)
		}

		if i.os != "" {
			config.Platform.OS = i.os
		}

		if i.arch != "" {
			config.Platform.Architecture = i.arch
		}

		if i.variant != "" {
			config.Platform.Variant = i.variant
		}

		if i.osVersion != "" {
			config.Platform.OSVersion = i.osVersion
		}

		mfest.Config = config
	})
	if err != nil {
		return err
	}

	// save
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

	return remote.Write(ref, i.image,
		remote.WithAuth(auth),
		remote.WithTransport(imgutil.GetTransport(reg.insecure)),
	)
}
