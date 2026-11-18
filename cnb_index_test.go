package imgutil_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/buildpacks/imgutil"
)

func TestCNBIndexSetIndexAnnotations(t *testing.T) {
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

			existingAnnotations := map[string]string{
				"existing-key": "existing-value",
				"shared-key":   "old-value",
			}
			if err := index.SetIndexAnnotations(existingAnnotations); err != nil {
				t.Fatal(err)
			}

			newAnnotations := map[string]string{
				"some-index-key": "some-index-value",
				"shared-key":     "new-value",
			}
			if err := index.SetIndexAnnotations(newAnnotations); err != nil {
				t.Fatal(err)
			}
			if err := index.SaveDir(); err != nil {
				t.Fatal(err)
			}

			expectedAnnotations := map[string]string{
				"existing-key":   "existing-value",
				"some-index-key": "some-index-value",
				"shared-key":     "new-value",
			}
			indexManifest := readIndexManifest(t, tmpDir, repoName)
			if diff := cmp.Diff(expectedAnnotations, indexManifest.Annotations); diff != "" {
				t.Fatalf("unexpected index annotations (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.expectedMediaType, indexManifest.MediaType); diff != "" {
				t.Fatalf("unexpected index media type (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(1, len(indexManifest.Manifests)); diff != "" {
				t.Fatalf("unexpected manifest count (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCNBIndexSaveDirWithoutIndexAnnotations(t *testing.T) {
	tmpDir := t.TempDir()
	repoName := "some/index"

	index, err := imgutil.NewCNBIndex(repoName, imgutil.IndexOptions{
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

	if err := index.SaveDir(); err != nil {
		t.Fatal(err)
	}

	indexManifest := readIndexManifest(t, tmpDir, repoName)
	if diff := cmp.Diff(map[string]string(nil), indexManifest.Annotations); diff != "" {
		t.Fatalf("unexpected index annotations (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(1, len(indexManifest.Manifests)); diff != "" {
		t.Fatalf("unexpected manifest count (-want +got):\n%s", diff)
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
