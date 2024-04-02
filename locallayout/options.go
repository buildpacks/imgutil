package locallayout

import (
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/imgutil"
)

func FromBaseImage(name string) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.BaseImageRepoName = name
	}
}

func WithConfig(c *v1.Config) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.Config = c
	}
}

func WithCreatedAt(t time.Time) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.CreatedAt = t
	}
}

func WithDefaultPlatform(p v1.Platform) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.Platform = p
	}
}

func WithHistory() func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.PreserveHistory = true
	}
}

func WithPreviousImage(name string) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.PreviousImageRepoName = name
	}
}

func WithMediaTypes(m imgutil.MediaTypes) func(*imgutil.ImageOptions) {
	return func(o *imgutil.ImageOptions) {
		o.MediaTypes = m
	}
}
