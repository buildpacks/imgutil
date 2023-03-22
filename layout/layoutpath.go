package layout

import (
	"path/filepath"

	ggcr "github.com/google/go-containerregistry/pkg/v1/layout"
)

type Path struct {
	ggcr.Path
}

type AppendOption func(*appendOptions)

type appendOptions struct {
	withoutLayers bool
	annotations   map[string]string
}

func WithoutLayers() AppendOption {
	return func(i *appendOptions) {
		i.withoutLayers = true
	}
}

func WithAnnotations(annotations map[string]string) AppendOption {
	return func(i *appendOptions) {
		i.annotations = annotations
	}
}

func (l Path) append(elem ...string) string {
	complete := []string{string(l.Path)}
	return filepath.Join(append(complete, elem...)...)
}
