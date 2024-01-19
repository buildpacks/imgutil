package imgutil

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type IndexAddOption func(*AddOptions) error
type IndexPushOption func(*PushOptions) error
type IndexOption func(*IndexOptions) error

type AddOptions struct {
	all bool
	os, arch, variant, osVersion string
	features, osFeatures []string
	annotations map[string]string
}

type PushOptions struct {
	insecure, purge bool
	format types.MediaType
}

type IndexOptions struct {
	KeyChain  authn.Keychain
	XdgPath, Reponame string
	InsecureRegistry bool
}

func(o *IndexOptions) Keychain() authn.Keychain {
	return o.KeyChain
}

func(o *IndexOptions) XDGRuntimePath() string {
	return o.XdgPath
}

func(o *IndexOptions) RepoName() string {
	return o.Reponame
}

func(o *IndexOptions) Insecure() bool {
	return o.InsecureRegistry
}

func WithKeychain(keychain authn.Keychain) IndexOption {
	return func(o *IndexOptions) error {
		o.KeyChain = keychain
		return nil
	}
}

func WithXDGRuntimePath(xdgPath string) IndexOption {
	return func(o *IndexOptions) error {
		o.XdgPath = xdgPath
		return nil
	}
}

func WithRepoName(repoName string) IndexOption {
	return func(o *IndexOptions) error {
		if o.InsecureRegistry {
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
		o.Reponame = repoName
		return nil
	}
}

func WithInsecureIndex(insecure bool) IndexOption {
	return func(o *IndexOptions) error {
		o.InsecureRegistry = insecure
		return nil
	}
}

func WithAll(all bool) IndexAddOption {
	return func(a *AddOptions) error {
		a.all = all
		return nil
	}
}

func WithOS(os string) IndexAddOption {
	return func(a *AddOptions) error {
		a.os = os
		return nil
	}
}

func WithArchitecture(arch string) IndexAddOption {
	return func(a *AddOptions) error {
		a.arch = arch
		return nil
	}
}

func WithVariant(variant string) IndexAddOption {
	return func(a *AddOptions) error {
		a.variant = variant
		return nil
	}
}

func WithOSVersion(osVersion string) IndexAddOption {
	return func(a *AddOptions) error {
		a.osVersion = osVersion
		return nil
	}
}

func WithFeatures(features []string) IndexAddOption {
	return func(a *AddOptions) error {
		a.features = features
		return nil
	}
}

func WithOSFeatures(osFeatures []string) IndexAddOption {
	return func(a *AddOptions) error {
		a.osFeatures = osFeatures
		return nil
	}
}

func WithInsecure(insecure bool) IndexPushOption {
	return func(a *PushOptions) error {
		a.insecure = insecure
		return nil
	}
}

func WithPurge(purge bool) IndexPushOption {
	return func(a *PushOptions) error {
		a.purge = purge
		return nil
	}
}

func WithFormat(format types.MediaType) IndexPushOption {
	return func(a *PushOptions) error {
		a.format = format
		return nil
	}
}

func WithAnnotations(annotations map[string]string) IndexAddOption {
	return func(a *AddOptions) error {
		a.annotations = annotations
		return nil
	}
}