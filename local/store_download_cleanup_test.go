package local

import (
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestCleanupDownloadDirs(t *testing.T) {
	s := NewStore(nil)

	downloadDir, err := os.MkdirTemp("", "imgutil.local.image.")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	// A second dir that simulates a pack/builder-owned layer path (must not be removed).
	otherDir, err := os.MkdirTemp("", "create-builder-scratch")
	if err != nil {
		t.Fatalf("MkdirTemp other: %v", err)
	}
	defer os.RemoveAll(otherDir)

	marker := filepath.Join(downloadDir, "layer.tar")
	if err := os.WriteFile(marker, []byte("layer-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	otherFile := filepath.Join(otherDir, "new-layer.tar")
	if err := os.WriteFile(otherFile, []byte("new-layer"), 0o600); err != nil {
		t.Fatalf("WriteFile other: %v", err)
	}

	downloadedID := v1.Hash{Algorithm: "sha256", Hex: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	keptID := v1.Hash{Algorithm: "sha256", Hex: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}

	s.downloadDirs = []string{downloadDir}
	s.downloadedDiffIDs = []v1.Hash{downloadedID}
	s.onDiskLayersByDiffID[downloadedID] = annotatedLayer{}
	s.onDiskLayersByDiffID[keptID] = annotatedLayer{}

	s.cleanupDownloadDirs()

	if _, err := os.Stat(downloadDir); !os.IsNotExist(err) {
		t.Fatalf("expected download dir to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(otherFile); err != nil {
		t.Fatalf("intentional cache/artifact path should remain: %v", err)
	}
	if _, ok := s.onDiskLayersByDiffID[downloadedID]; ok {
		t.Fatal("expected downloaded layer handle to be dropped")
	}
	if _, ok := s.onDiskLayersByDiffID[keptID]; !ok {
		t.Fatal("expected non-download layer handle to be kept")
	}
	if len(s.downloadDirs) != 0 || len(s.downloadedDiffIDs) != 0 {
		t.Fatal("expected download tracking slices to be cleared")
	}
}

func TestPartialDownloadDropsLayerHandles(t *testing.T) {
	s := NewStore(nil)

	downloadedID := v1.Hash{Algorithm: "sha256", Hex: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	keptID := v1.Hash{Algorithm: "sha256", Hex: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	s.onDiskLayersByDiffID[downloadedID] = annotatedLayer{}
	s.onDiskLayersByDiffID[keptID] = annotatedLayer{}

	// Simulate AddLayer succeeding for the first extracted layer, then a later
	// layer failing before downloadedDiffIDs is updated.
	s.dropDownloadedLayerHandles([]v1.Hash{downloadedID})

	if _, ok := s.onDiskLayersByDiffID[downloadedID]; ok {
		t.Fatal("expected layer from a failed download to be dropped")
	}
	if _, ok := s.onDiskLayersByDiffID[keptID]; !ok {
		t.Fatal("expected unrelated layer handle to be kept")
	}
}

func TestCleanupDownloadDirsNoopWhenEmpty(t *testing.T) {
	s := NewStore(nil)
	// Should not panic or reset state when nothing was downloaded.
	s.cleanupDownloadDirs()
	if s.downloadOnce == nil {
		t.Fatal("downloadOnce should remain usable")
	}
}
