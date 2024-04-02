package layout

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"

	"github.com/buildpacks/imgutil"
)

func (i *Image) Save(additionalNames ...string) error {
	return i.SaveAs(i.Name(), additionalNames...)
}

// SaveAs ignores the image `Name()` method and saves the image according to name & additional names provided to this method
func (i *Image) SaveAs(name string, additionalNames ...string) error {
	if !i.preserveDigest {
		if err := i.SetCreatedAtAndHistory(); err != nil {
			return err
		}
	}

	refName, err := i.GetAnnotateRefName()
	if err != nil {
		return err
	}
	ops := []AppendOption{WithAnnotations(ImageRefAnnotation(refName))}
	if i.saveWithoutLayers {
		ops = append(ops, WithoutLayers())
	}

	i.Image, err = imgutil.MutateManifest(i, func(mfest *v1.Manifest) {
		config := mfest.Config
		if annos, _ := i.Annotations(); len(annos) != 0 {
			mfest.Annotations = annos
			config.Annotations = annos
		}

		if urls, _ := i.URLs(); len(urls) != 0 {
			config.URLs = append(config.URLs, urls...)
		}

		if config.Platform == nil {
			config.Platform = &v1.Platform{}
		}

		if features, _ := i.Features(); len(features) != 0 {
			config.Platform.Features = append(config.Platform.Features, features...)
		}

		if osFeatures, _ := i.OSFeatures(); len(osFeatures) != 0 {
			config.Platform.OSFeatures = append(config.Platform.OSFeatures, osFeatures...)
		}

		if os, _ := i.OS(); os != "" {
			config.Platform.OS = os
		}

		if arch, _ := i.Architecture(); arch != "" {
			config.Platform.Architecture = arch
		}

		if variant, _ := i.Variant(); variant != "" {
			config.Platform.Variant = variant
		}

		if osVersion, _ := i.OSVersion(); osVersion != "" {
			config.Platform.OSVersion = osVersion
		}

		mfest.Config = config
	})
	if err != nil {
		return err
	}

	var (
		pathsToSave = append([]string{name}, additionalNames...)
		diagnostics []imgutil.SaveDiagnostic
	)
	for _, path := range pathsToSave {
		layoutPath, err := initEmptyIndexAt(path)
		if err != nil {
			return err
		}
		if err = layoutPath.AppendImage(
			i.Image,
			ops...,
		); err != nil {
			diagnostics = append(diagnostics, imgutil.SaveDiagnostic{ImageName: i.Name(), Cause: err})
		}
	}
	if len(diagnostics) > 0 {
		return imgutil.SaveError{Errors: diagnostics}
	}

	return nil
}

func initEmptyIndexAt(path string) (Path, error) {
	return Write(path, empty.Index)
}
