package index

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type IndexOption func(*IndexOptions) error

type IndexOptions struct {
	keychain          authn.Keychain
    xdgPath, repoName string
    insecure          bool
	format types.MediaType
}

func(o *IndexOptions) Keychain() authn.Keychain {
	return o.keychain
}

func(o *IndexOptions) XDGRuntimePath() string {
	return o.xdgPath
}

func(o *IndexOptions) RepoName() string {
	return o.repoName
}

func(o *IndexOptions) Insecure() bool {
	return o.insecure
}

func(o *IndexOptions) Format() types.MediaType {
	return o.format
}

func WithKeychain(keychain authn.Keychain) IndexOption {
	return func(o *IndexOptions) error {
		o.keychain = keychain
		return nil
	}
}

func WithXDGRuntimePath(xdgPath string) IndexOption {
	return func(o *IndexOptions) error {
		o.xdgPath = xdgPath
		return nil
	}
}

func WithRepoName(repoName string) IndexOption {
	return func(o *IndexOptions) error {
		if o.insecure {
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
		o.repoName = repoName
		return nil
	}
}

func WithInsecure(insecure bool) IndexOption {
	return func(o *IndexOptions) error {
		o.insecure = insecure
		return nil
	}
}

func WithFormat(format types.MediaType) IndexOption {
	return func(o *IndexOptions) error {
		o.format = format
		return nil
	}
}