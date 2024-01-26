package imgutil

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type IndexAddOption func(*AddOptions)
type IndexPushOption func(*PushOptions) error

type AddOptions struct {
	All                          bool
	OS, Arch, Variant, OSVersion string
	Features, OSFeatures         []string
	Annotations                  map[string]string
}

type PushOptions struct {
	Insecure, Purge bool
	Format          types.MediaType
}

type IndexOptions struct {
	KeyChain          authn.Keychain
	XdgPath, Reponame string
	InsecureRegistry  bool
}

func (o *IndexOptions) Keychain() authn.Keychain {
	return o.KeyChain
}

func (o *IndexOptions) XDGRuntimePath() string {
	return o.XdgPath
}

func (o *IndexOptions) RepoName() string {
	return o.Reponame
}

func (o *IndexOptions) Insecure() bool {
	return o.InsecureRegistry
}

func WithAll(all bool) IndexAddOption {
	return func(a *AddOptions) {
		a.All = all
	}
}

func WithOS(os string) IndexAddOption {
	return func(a *AddOptions) {
		a.OS = os
	}
}

func WithArchitecture(arch string) IndexAddOption {
	return func(a *AddOptions) {
		a.Arch = arch
	}
}

func WithVariant(variant string) IndexAddOption {
	return func(a *AddOptions) {
		a.Variant = variant
	}
}

func WithOSVersion(osVersion string) IndexAddOption {
	return func(a *AddOptions) {
		a.OSVersion = osVersion
	}
}

func WithFeatures(features []string) IndexAddOption {
	return func(a *AddOptions) {
		a.Features = features
	}
}

func WithOSFeatures(osFeatures []string) IndexAddOption {
	return func(a *AddOptions) {
		a.OSFeatures = osFeatures
	}
}

func WithAnnotations(annotations map[string]string) IndexAddOption {
	return func(a *AddOptions) {
		a.Annotations = annotations
	}
}

func WithInsecure(insecure bool) IndexPushOption {
	return func(a *PushOptions) error {
		a.Insecure = insecure
		return nil
	}
}

func WithPurge(purge bool) IndexPushOption {
	return func(a *PushOptions) error {
		a.Purge = purge
		return nil
	}
}

func WithFormat(format types.MediaType) IndexPushOption {
	return func(a *PushOptions) error {
		if !format.IsIndex() {
			return ErrUnknownMediaType
		}
		a.Format = format
		return nil
	}
}
