package index

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

type Option func(options *imgutil.IndexOptions) error
type PushOption func(*imgutil.IndexPushOptions) error
type AddOption func(*imgutil.IndexAddOptions) error

// WithKeychain fetches Index from registry with keychain
func WithKeychain(keychain authn.Keychain) Option {
	return func(o *imgutil.IndexOptions) error {
		o.KeyChain = keychain
		return nil
	}
}

// WithXDGRuntimePath Saves the Index to the '`xdgPath`/manifests'
func WithXDGRuntimePath(xdgPath string) Option {
	return func(o *imgutil.IndexOptions) error {
		o.XdgPath = xdgPath
		return nil
	}
}

// PullInsecure If true, pulls images from insecure registry
func PullInsecure() Option {
	return func(o *imgutil.IndexOptions) error {
		o.Insecure = true
		return nil
	}
}

// Push index to Insecure Registry
func WithInsecure(insecure bool) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		a.Insecure = insecure
		return nil
	}
}

// Push the Index with given format
func UsingFormat(format types.MediaType) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		if !format.IsIndex() {
			return imgutil.ErrUnknownMediaType(format)
		}
		a.Format = format
		return nil
	}
}

// UsingFormat Create the image index with the following format
func WithFormat(format types.MediaType) Option {
	return func(o *imgutil.IndexOptions) error {
		o.Format = format
		return nil
	}
}

// Others

// Add all images within the index
func WithAll(all bool) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.All = all
		return nil
	}
}

// Add a single image from index with given OS
func WithOS(os string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.OS = os
		return nil
	}
}

// Add a Local image to Index
func WithLocalImage(image imgutil.EditableImage) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Local = true
		a.Image = image
		return nil
	}
}

// Add a single image from index with given Architecture
func WithArchitecture(arch string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Arch = arch
		return nil
	}
}

// Add a single image from index with given Variant
func WithVariant(variant string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Variant = variant
		return nil
	}
}

// Add a single image from index with given OSVersion
func WithOSVersion(osVersion string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.OSVersion = osVersion
		return nil
	}
}

// Add a single image from index with given Features
func WithFeatures(features []string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Features = features
		return nil
	}
}

// Add a single image from index with given OSFeatures
func WithOSFeatures(osFeatures []string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.OSFeatures = osFeatures
		return nil
	}
}

// Add a single image from index with given Annotations
func WithAnnotations(annotations map[string]string) AddOption {
	return func(a *imgutil.IndexAddOptions) error {
		a.Annotations = annotations
		return nil
	}
}

// If true, Deletes index from local filesystem after pushing to registry
func WithPurge(purge bool) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		a.Purge = purge
		return nil
	}
}

// Push the Index with given format
func WithTags(tags ...string) PushOption {
	return func(a *imgutil.IndexPushOptions) error {
		a.Tags = tags
		return nil
	}
}

// ValidateRepoName
// TODO move this code to something more generic
func ValidateRepoName(repoName string, o *imgutil.IndexOptions) error {
	if o.Insecure {
		_, err := name.ParseReference(repoName, name.Insecure, name.WeakValidation)
		if err != nil {
			return err
		}
	} else {
		_, err := name.ParseReference(repoName, name.WeakValidation)
		if err != nil {
			return err
		}
	}
	o.BaseImageIndexRepoName = repoName
	return nil
}
