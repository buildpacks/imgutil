package index

import "github.com/buildpacks/imgutil"

var _ imgutil.ImageIndex = (*ImageIndex)(nil)

type ImageIndex struct {
	*imgutil.CNBIndex
}
