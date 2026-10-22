package imgutil_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

// TestCNBIndexSetAnnotationsPreservesMediaType is a regression test: replaceDescriptor
// (shared by SetOS, SetArchitecture, SetVariant, and SetAnnotations) used to compare a
// manifest entry's *image* media type against the index's own media type to decide
// whether to restore it after mutating the index - those are never equal, so an OCI
// index always ended up rewritten with a media type that was neither OCI nor Docker,
// and downstream code that checks for exactly types.OCIImageIndex treated it as Docker.
func TestCNBIndexSetAnnotationsPreservesMediaType(t *testing.T) {
	for _, test := range []struct {
		name              string
		mediaType         types.MediaType
		expectedMediaType types.MediaType
	}{
		{
			name:              "oci index",
			expectedMediaType: types.OCIImageIndex,
		},
		{
			name:              "docker index",
			mediaType:         types.DockerManifestList,
			expectedMediaType: types.DockerManifestList,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			repoName := "some/index"

			index, err := imgutil.NewCNBIndex(repoName, imgutil.IndexOptions{
				MediaType: test.mediaType,
				LayoutIndexOptions: imgutil.LayoutIndexOptions{
					XdgPath: tmpDir,
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			image, err := random.Image(1024, 1)
			if err != nil {
				t.Fatal(err)
			}
			index.AddManifest(image)

			hash, err := image.Digest()
			if err != nil {
				t.Fatal(err)
			}
			digest, err := name.NewDigest(fmt.Sprintf("random@%s", hash.String()))
			if err != nil {
				t.Fatal(err)
			}

			if err := index.SetAnnotations(digest, map[string]string{"some-key": "some-value"}); err != nil {
				t.Fatal(err)
			}
			if err := index.SaveDir(); err != nil {
				t.Fatal(err)
			}

			indexManifest := readIndexManifest(t, tmpDir, repoName)
			if diff := cmp.Diff(test.expectedMediaType, indexManifest.MediaType); diff != "" {
				t.Fatalf("unexpected index media type (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(1, len(indexManifest.Manifests)); diff != "" {
				t.Fatalf("unexpected manifest count (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff("some-value", indexManifest.Manifests[0].Annotations["some-key"]); diff != "" {
				t.Fatalf("unexpected manifest annotation (-want +got):\n%s", diff)
			}
		})
	}
}

func readIndexManifest(t *testing.T, tmpDir, repoName string) v1.IndexManifest {
	t.Helper()

	rawIndex, err := os.ReadFile(filepath.Join(tmpDir, imgutil.MakeFileSafeName(repoName), "index.json"))
	if err != nil {
		t.Fatal(err)
	}

	var indexManifest v1.IndexManifest
	if err := json.Unmarshal(rawIndex, &indexManifest); err != nil {
		t.Fatal(err)
	}

	return indexManifest
}
