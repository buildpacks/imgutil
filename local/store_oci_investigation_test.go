package local_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"

	"github.com/buildpacks/imgutil/local"
	h "github.com/buildpacks/imgutil/testhelpers"
)

// TestInvestigateImageLoadFormats tests whether Docker's ImageLoad API accepts
// different tar formats, particularly compressed layers and OCI layout format.
// This is an investigation test to determine optimization opportunities for
// containerd-snapshotter performance.
//
// Run with: go test -v -run TestInvestigateImageLoadFormats -count=1 ./local/
func TestInvestigateImageLoadFormats(t *testing.T) {
	docker := h.DockerCli(t)

	// Check if we're using containerd-snapshotter
	infoResult, err := docker.Info(context.TODO(), client.InfoOptions{})
	if err != nil {
		t.Fatalf("Failed to get docker info: %v", err)
	}
	isContainerd := false
	storageDriver := "unknown"
	for _, driverStatus := range infoResult.Info.DriverStatus {
		if driverStatus[0] == "driver-type" {
			storageDriver = driverStatus[1]
			if driverStatus[1] == "io.containerd.snapshotter.v1" {
				isContainerd = true
			}
		}
	}
	t.Logf("Storage driver: %s (containerd-snapshotter: %v)", storageDriver, isContainerd)

	// Ensure base image is available
	baseImageName := h.RunnableBaseImage("linux")
	h.PullIfMissing(t, docker, baseImageName)

	// Get the base image info for creating test images
	inspectResult, err := docker.ImageInspect(context.Background(), baseImageName)
	if err != nil {
		t.Fatalf("Failed to inspect base image: %v", err)
	}
	baseImageID := inspectResult.InspectResponse.ID
	t.Logf("Base image: %s (ID: %s, layers: %d)", baseImageName, baseImageID, len(inspectResult.InspectResponse.RootFS.Layers))

	// Extract layers from the base image via docker save
	t.Log("--- Extracting layers from base image ---")
	extractStart := time.Now()
	layers := extractImageLayers(t, docker, baseImageID)
	t.Logf("Extracted %d layers in %v", len(layers), time.Since(extractStart))
	for i, l := range layers {
		t.Logf("  Layer %d: diffID=%s uncompressedSize=%d compressedSize=%d",
			i, l.diffID, l.uncompressedSize, l.compressedSize)
	}

	// Test 1: Baseline - current imgutil Save() path
	t.Run("Baseline_CurrentSavePath", func(t *testing.T) {
		testBaselineSave(t, docker, baseImageName)
	})

	// Test 2: Docker legacy tar with uncompressed layers (current format)
	t.Run("DockerTar_UncompressedLayers", func(t *testing.T) {
		testDockerTarLoad(t, docker, layers, inspectResult.InspectResponse, false)
	})

	// Test 3: Docker legacy tar with compressed (gzip) layers
	t.Run("DockerTar_CompressedLayers", func(t *testing.T) {
		testDockerTarLoad(t, docker, layers, inspectResult.InspectResponse, true)
	})

	// Test 4: OCI layout tar with compressed layers
	t.Run("OCILayout_CompressedLayers", func(t *testing.T) {
		testOCILayoutLoad(t, docker, layers, inspectResult.InspectResponse)
	})

	// Test 5: Multi-layer image with compressed layers (more realistic)
	t.Run("DockerTar_CompressedMultiLayer", func(t *testing.T) {
		testCompressedMultiLayerImage(t, docker, layers, inspectResult.InspectResponse)
	})

	// Test 6: Timing breakdown of individual operations
	t.Run("TimingBreakdown", func(t *testing.T) {
		testTimingBreakdown(t, docker, baseImageName, baseImageID)
	})
}

type extractedLayer struct {
	diffID           string
	uncompressedData []byte
	compressedData   []byte
	uncompressedSize int64
	compressedSize   int64
}

func extractImageLayers(t *testing.T, docker client.APIClient, imageID string) []extractedLayer {
	t.Helper()

	ctx := context.Background()
	saveResult, err := docker.ImageSave(ctx, []string{imageID})
	if err != nil {
		t.Fatalf("ImageSave failed: %v", err)
	}
	defer saveResult.Close()

	// Extract tar to temp dir
	tmpDir, err := os.MkdirTemp("", "imgutil-investigation-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tr := tar.NewReader(saveResult)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}
		path := filepath.Join(tmpDir, filepath.Clean(hdr.Name))
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(path, 0750)
			continue
		}
		os.MkdirAll(filepath.Dir(path), 0750)
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create file: %v", err)
		}
		io.Copy(f, tr)
		f.Close()
	}

	// Read manifest
	mfData, err := os.ReadFile(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest []struct {
		Config string
		Layers []string
	}
	if err := json.Unmarshal(mfData, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(manifest) != 1 {
		t.Fatalf("expected 1 manifest entry, got %d", len(manifest))
	}

	// Read config to get diffIDs
	cfgData, err := os.ReadFile(filepath.Join(tmpDir, manifest[0].Config))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var config struct {
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err := json.Unmarshal(cfgData, &config); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	// Read each layer
	var layers []extractedLayer
	for idx, layerPath := range manifest[0].Layers {
		layerData, err := os.ReadFile(filepath.Join(tmpDir, layerPath))
		if err != nil {
			t.Fatalf("read layer %d: %v", idx, err)
		}

		// Compress the layer
		var compressedBuf bytes.Buffer
		gzWriter := gzip.NewWriter(&compressedBuf)
		gzWriter.Write(layerData)
		gzWriter.Close()

		layers = append(layers, extractedLayer{
			diffID:           config.RootFS.DiffIDs[idx],
			uncompressedData: layerData,
			compressedData:   compressedBuf.Bytes(),
			uncompressedSize: int64(len(layerData)),
			compressedSize:   int64(compressedBuf.Len()),
		})
	}
	return layers
}

func testBaselineSave(t *testing.T, docker client.APIClient, baseImageName string) {
	t.Helper()

	// Create a new image from the base and save it (mimics the imgutil Save path)
	repoName := "imgutil-investigation-baseline-" + h.RandString(10)
	defer h.DockerRmi(docker, repoName)

	start := time.Now()
	img, err := local.NewImage(repoName, docker, local.FromBaseImage(baseImageName))
	if err != nil {
		t.Fatalf("NewImage: %v", err)
	}
	createDuration := time.Since(start)

	// Add a small layer to make it non-trivial
	layerPath, err := h.CreateSingleFileLayerTar("/test-file.txt", "investigation test content", "linux")
	if err != nil {
		t.Fatalf("CreateSingleFileLayerTar: %v", err)
	}
	defer os.Remove(layerPath)

	if err := img.AddLayer(layerPath); err != nil {
		t.Fatalf("AddLayer: %v", err)
	}

	start = time.Now()
	if err := img.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saveDuration := time.Since(start)

	t.Logf("BASELINE: create=%v save=%v total=%v", createDuration, saveDuration, createDuration+saveDuration)

	// Verify the image
	inspectResult, err := docker.ImageInspect(context.Background(), repoName)
	if err != nil {
		t.Fatalf("ImageInspect after save: %v", err)
	}
	t.Logf("BASELINE: saved image has %d layers", len(inspectResult.InspectResponse.RootFS.Layers))
}

func testDockerTarLoad(t *testing.T, docker client.APIClient, layers []extractedLayer, baseInspect image.InspectResponse, compressed bool) {
	t.Helper()

	repoName := fmt.Sprintf("imgutil-investigation-docker-%s-%s:latest",
		map[bool]string{true: "compressed", false: "uncompressed"}[compressed],
		h.RandString(10))
	defer h.DockerRmi(docker, repoName)

	// Build the config file
	configFile := buildConfigFile(baseInspect, layers)
	configJSON, err := json.Marshal(configFile)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configDigest := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	// Build the tar
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Add config
	addToTar(t, tw, configDigest+".json", configJSON)

	// Add layers
	var layerPaths []string
	for _, l := range layers {
		var layerName string
		var layerData []byte
		if compressed {
			layerName = fmt.Sprintf("%s.tar.gz", l.diffID)
			layerData = l.compressedData
		} else {
			layerName = fmt.Sprintf("%s.tar", l.diffID)
			layerData = l.uncompressedData
		}
		addToTar(t, tw, layerName, layerData)
		layerPaths = append(layerPaths, layerName)
	}

	// Add manifest.json
	manifestJSON, err := json.Marshal([]map[string]interface{}{
		{
			"Config":   configDigest + ".json",
			"RepoTags": []string{repoName},
			"Layers":   layerPaths,
		},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	addToTar(t, tw, "manifest.json", manifestJSON)
	tw.Close()

	// Load the tar
	loadStart := time.Now()
	loadResult, err := docker.ImageLoad(context.Background(), bytes.NewReader(buf.Bytes()), client.ImageLoadWithQuiet(true))
	if err != nil {
		t.Fatalf("ImageLoad failed: %v", err)
	}
	responseBody, _ := io.ReadAll(loadResult)
	loadResult.Close()
	loadDuration := time.Since(loadStart)

	t.Logf("FORMAT=%s: load=%v response=%s",
		map[bool]string{true: "compressed", false: "uncompressed"}[compressed],
		loadDuration, string(responseBody))

	// Validate
	inspectResult, err := docker.ImageInspect(context.Background(), repoName)
	if err != nil {
		t.Logf("FORMAT=%s: FAILED - image not found after load: %v",
			map[bool]string{true: "compressed", false: "uncompressed"}[compressed], err)
		return
	}
	t.Logf("FORMAT=%s: SUCCESS - loaded image has %d layers, ID=%s",
		map[bool]string{true: "compressed", false: "uncompressed"}[compressed],
		len(inspectResult.InspectResponse.RootFS.Layers), inspectResult.InspectResponse.ID)

	// Validate layer diffIDs match
	for i, expectedDiffID := range layers {
		if i >= len(inspectResult.InspectResponse.RootFS.Layers) {
			t.Logf("FORMAT=%s: MISMATCH - fewer layers than expected",
				map[bool]string{true: "compressed", false: "uncompressed"}[compressed])
			break
		}
		actualDiffID := inspectResult.InspectResponse.RootFS.Layers[i]
		if actualDiffID != expectedDiffID.diffID {
			t.Logf("FORMAT=%s: MISMATCH at layer %d: expected %s, got %s",
				map[bool]string{true: "compressed", false: "uncompressed"}[compressed],
				i, expectedDiffID.diffID, actualDiffID)
		}
	}
}

func testOCILayoutLoad(t *testing.T, docker client.APIClient, layers []extractedLayer, baseInspect image.InspectResponse) {
	t.Helper()

	repoName := "imgutil-investigation-oci-" + h.RandString(10) + ":latest"
	defer h.DockerRmi(docker, repoName)

	// Build OCI config
	configFile := buildConfigFile(baseInspect, layers)
	configJSON, err := json.Marshal(configFile)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(configJSON))

	// Compute layer descriptors
	type ociDescriptor struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int64  `json:"size"`
	}

	var layerDescs []ociDescriptor
	layerBlobs := make(map[string][]byte)
	for _, l := range layers {
		digest := fmt.Sprintf("sha256:%x", sha256.Sum256(l.compressedData))
		layerDescs = append(layerDescs, ociDescriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    digest,
			Size:      l.compressedSize,
		})
		layerBlobs[digest] = l.compressedData
	}

	// Build OCI manifest
	ociManifest := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config": ociDescriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      int64(len(configJSON)),
		},
		"layers": layerDescs,
	}
	manifestJSON, err := json.Marshal(ociManifest)
	if err != nil {
		t.Fatalf("marshal oci manifest: %v", err)
	}
	manifestDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(manifestJSON))

	// Build OCI index
	ociIndex := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.index.v1+json",
		"manifests": []map[string]interface{}{
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest":    manifestDigest,
				"size":      int64(len(manifestJSON)),
				"annotations": map[string]string{
					"io.containerd.image.name": repoName,
					"org.opencontainers.image.ref.name": func() string {
						// Extract just the tag
						for i := len(repoName) - 1; i >= 0; i-- {
							if repoName[i] == ':' {
								return repoName[i+1:]
							}
						}
						return "latest"
					}(),
				},
			},
		},
	}
	indexJSON, err := json.Marshal(ociIndex)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}

	// Build the tar with OCI layout structure
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// oci-layout file
	addToTar(t, tw, "oci-layout", []byte(`{"imageLayoutVersion":"1.0.0"}`))

	// index.json
	addToTar(t, tw, "index.json", indexJSON)

	// blobs/sha256/<manifest>
	addBlobToTar(t, tw, manifestDigest, manifestJSON)

	// blobs/sha256/<config>
	addBlobToTar(t, tw, configDigest, configJSON)

	// blobs/sha256/<layers>
	for digest, data := range layerBlobs {
		addBlobToTar(t, tw, digest, data)
	}

	tw.Close()

	// Load the OCI layout tar
	loadStart := time.Now()
	loadResult, err := docker.ImageLoad(context.Background(), bytes.NewReader(buf.Bytes()), client.ImageLoadWithQuiet(true))
	if err != nil {
		t.Fatalf("ImageLoad (OCI) failed: %v", err)
	}
	responseBody, _ := io.ReadAll(loadResult)
	loadResult.Close()
	loadDuration := time.Since(loadStart)

	t.Logf("FORMAT=oci-layout: load=%v response=%s", loadDuration, string(responseBody))

	// Validate - try by name first, then try to find by config digest
	inspectResult, err := docker.ImageInspect(context.Background(), repoName)
	if err != nil {
		// Try with docker.io/library/ prefix
		fullName := "docker.io/library/" + repoName
		inspectResult, err = docker.ImageInspect(context.Background(), fullName)
		if err != nil {
			// Try finding by config digest
			inspectResult, err = docker.ImageInspect(context.Background(), configDigest)
			if err != nil {
				t.Logf("FORMAT=oci-layout: FAILED - image not found by name (%s), full name (%s), or config digest (%s): %v",
					repoName, fullName, configDigest, err)
				return
			}
			t.Logf("FORMAT=oci-layout: found by config digest (not by tag)")
		} else {
			t.Logf("FORMAT=oci-layout: found by full name (docker.io/library/...)")
		}
	}
	t.Logf("FORMAT=oci-layout: SUCCESS - loaded image has %d layers, ID=%s",
		len(inspectResult.InspectResponse.RootFS.Layers), inspectResult.InspectResponse.ID)

	// Validate layer diffIDs
	for i, expectedLayer := range layers {
		if i >= len(inspectResult.InspectResponse.RootFS.Layers) {
			t.Logf("FORMAT=oci-layout: MISMATCH - fewer layers than expected")
			break
		}
		actualDiffID := inspectResult.InspectResponse.RootFS.Layers[i]
		if actualDiffID != expectedLayer.diffID {
			t.Logf("FORMAT=oci-layout: MISMATCH at layer %d: expected %s, got %s",
				i, expectedLayer.diffID, actualDiffID)
		}
	}
}

func testCompressedMultiLayerImage(t *testing.T, docker client.APIClient, baseLayers []extractedLayer, baseInspect image.InspectResponse) {
	t.Helper()

	// Create additional layers to simulate a realistic multi-layer image
	// (base layers + buildpack layers + app layer)
	allLayers := make([]extractedLayer, len(baseLayers))
	copy(allLayers, baseLayers)

	for i := 0; i < 5; i++ {
		content := fmt.Sprintf("buildpack-layer-%d-content-%s", i, h.RandString(1000))
		layerPath, err := h.CreateSingleFileLayerTar(
			fmt.Sprintf("/workspace/layer-%d.txt", i),
			content, "linux",
		)
		if err != nil {
			t.Fatalf("create test layer: %v", err)
		}
		defer os.Remove(layerPath)

		layerData, err := os.ReadFile(layerPath)
		if err != nil {
			t.Fatalf("read test layer: %v", err)
		}

		diffID := fmt.Sprintf("sha256:%x", sha256.Sum256(layerData))

		var compressedBuf bytes.Buffer
		gzWriter := gzip.NewWriter(&compressedBuf)
		gzWriter.Write(layerData)
		gzWriter.Close()

		allLayers = append(allLayers, extractedLayer{
			diffID:           diffID,
			uncompressedData: layerData,
			compressedData:   compressedBuf.Bytes(),
			uncompressedSize: int64(len(layerData)),
			compressedSize:   int64(compressedBuf.Len()),
		})
	}

	t.Logf("Testing with %d layers (%d base + 5 added)", len(allLayers), len(baseLayers))

	// Test uncompressed load
	repoUncompressed := "imgutil-investigation-multi-uncomp-" + h.RandString(10) + ":latest"
	defer h.DockerRmi(docker, repoUncompressed)

	tarUncompressed := buildDockerTar(t, allLayers, baseInspect, repoUncompressed, false)
	start := time.Now()
	loadResult, err := docker.ImageLoad(context.Background(), bytes.NewReader(tarUncompressed), client.ImageLoadWithQuiet(true))
	if err != nil {
		t.Fatalf("ImageLoad (uncompressed multi) failed: %v", err)
	}
	io.ReadAll(loadResult)
	loadResult.Close()
	uncompressedDuration := time.Since(start)

	// Test compressed load
	repoCompressed := "imgutil-investigation-multi-comp-" + h.RandString(10) + ":latest"
	defer h.DockerRmi(docker, repoCompressed)

	tarCompressed := buildDockerTar(t, allLayers, baseInspect, repoCompressed, true)
	start = time.Now()
	loadResult, err = docker.ImageLoad(context.Background(), bytes.NewReader(tarCompressed), client.ImageLoadWithQuiet(true))
	if err != nil {
		t.Fatalf("ImageLoad (compressed multi) failed: %v", err)
	}
	io.ReadAll(loadResult)
	loadResult.Close()
	compressedDuration := time.Since(start)

	t.Logf("MULTI-LAYER (%d layers):", len(allLayers))
	t.Logf("  uncompressed tar size: %d bytes, load time: %v", len(tarUncompressed), uncompressedDuration)
	t.Logf("  compressed tar size:   %d bytes, load time: %v", len(tarCompressed), compressedDuration)
	t.Logf("  size reduction: %.1f%%", (1.0-float64(len(tarCompressed))/float64(len(tarUncompressed)))*100)

	// Validate both images
	for _, name := range []string{repoUncompressed, repoCompressed} {
		inspectResult, err := docker.ImageInspect(context.Background(), name)
		if err != nil {
			t.Logf("  %s: FAILED to inspect: %v", name, err)
			continue
		}
		actualLayerCount := len(inspectResult.InspectResponse.RootFS.Layers)
		if actualLayerCount != len(allLayers) {
			t.Logf("  %s: MISMATCH - expected %d layers, got %d", name, len(allLayers), actualLayerCount)
		} else {
			t.Logf("  %s: OK - %d layers", name, actualLayerCount)
			// Verify all diff IDs
			for i, expected := range allLayers {
				actual := inspectResult.InspectResponse.RootFS.Layers[i]
				if actual != expected.diffID {
					t.Logf("  %s: MISMATCH at layer %d: expected %s, got %s", name, i, expected.diffID, actual)
				}
			}
		}
	}
}

func buildDockerTar(t *testing.T, layers []extractedLayer, baseInspect image.InspectResponse, repoName string, compressed bool) []byte {
	t.Helper()

	configFile := buildConfigFile(baseInspect, layers)
	configJSON, err := json.Marshal(configFile)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configDigest := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	addToTar(t, tw, configDigest+".json", configJSON)

	var layerPaths []string
	for _, l := range layers {
		var layerName string
		var layerData []byte
		if compressed {
			layerName = fmt.Sprintf("%s.tar.gz", l.diffID)
			layerData = l.compressedData
		} else {
			layerName = fmt.Sprintf("%s.tar", l.diffID)
			layerData = l.uncompressedData
		}
		addToTar(t, tw, layerName, layerData)
		layerPaths = append(layerPaths, layerName)
	}

	manifestJSON, _ := json.Marshal([]map[string]interface{}{
		{
			"Config":   configDigest + ".json",
			"RepoTags": []string{repoName},
			"Layers":   layerPaths,
		},
	})
	addToTar(t, tw, "manifest.json", manifestJSON)
	tw.Close()
	return buf.Bytes()
}

func testTimingBreakdown(t *testing.T, docker client.APIClient, baseImageName, baseImageID string) {
	t.Helper()

	// Measure usesContainerdStorage (docker.Info)
	start := time.Now()
	for i := 0; i < 5; i++ {
		docker.Info(context.Background(), client.InfoOptions{})
	}
	infoDuration := time.Since(start) / 5
	t.Logf("TIMING: docker.Info() avg=%v (per call)", infoDuration)

	// Measure ImageSave (layer download)
	start = time.Now()
	saveResult, err := docker.ImageSave(context.Background(), []string{baseImageID})
	if err != nil {
		t.Fatalf("ImageSave: %v", err)
	}
	savedData, _ := io.ReadAll(saveResult)
	saveResult.Close()
	imageSaveDuration := time.Since(start)
	t.Logf("TIMING: ImageSave()=%v (size=%d bytes)", imageSaveDuration, len(savedData))

	// Measure untar
	tmpDir, _ := os.MkdirTemp("", "imgutil-timing-*")
	defer os.RemoveAll(tmpDir)
	start = time.Now()
	tr := tar.NewReader(bytes.NewReader(savedData))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		path := filepath.Join(tmpDir, filepath.Clean(hdr.Name))
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(path, 0750)
			continue
		}
		os.MkdirAll(filepath.Dir(path), 0750)
		f, _ := os.Create(path)
		io.Copy(f, tr)
		f.Close()
	}
	untarDuration := time.Since(start)
	t.Logf("TIMING: untar=%v", untarDuration)

	// Measure decompression of a layer (if compressed on disk)
	mfData, _ := os.ReadFile(filepath.Join(tmpDir, "manifest.json"))
	var manifest []struct {
		Config string
		Layers []string
	}
	json.Unmarshal(mfData, &manifest)
	if len(manifest) > 0 && len(manifest[0].Layers) > 0 {
		layerPath := filepath.Join(tmpDir, manifest[0].Layers[0])
		layerData, _ := os.ReadFile(layerPath)

		// Check if layer is compressed
		isCompressed := len(layerData) > 2 && layerData[0] == 0x1f && layerData[1] == 0x8b

		if isCompressed {
			// Time decompression
			start = time.Now()
			gzReader, err := gzip.NewReader(bytes.NewReader(layerData))
			if err == nil {
				decompressed, _ := io.ReadAll(gzReader)
				gzReader.Close()
				decompressDuration := time.Since(start)
				t.Logf("TIMING: decompress layer[0]=%v (compressed=%d, uncompressed=%d, ratio=%.1fx)",
					decompressDuration, len(layerData), len(decompressed), float64(len(decompressed))/float64(len(layerData)))
			}
		} else {
			t.Logf("TIMING: layer[0] is already uncompressed (size=%d)", len(layerData))
		}
	}

	// Measure ImageLoad with a pre-built tar
	repoName := "imgutil-timing-test-" + h.RandString(10)
	defer h.DockerRmi(docker, repoName)

	img, err := local.NewImage(repoName, docker, local.FromBaseImage(baseImageName))
	if err != nil {
		t.Fatalf("NewImage: %v", err)
	}
	layerPath, _ := h.CreateSingleFileLayerTar("/timing-test.txt", "timing", "linux")
	defer os.Remove(layerPath)
	img.AddLayer(layerPath)

	start = time.Now()
	img.Save()
	fullSaveDuration := time.Since(start)
	t.Logf("TIMING: full Save() with 1 added layer=%v", fullSaveDuration)

	t.Log("--- Summary ---")
	t.Logf("  docker.Info()     : %v", infoDuration)
	t.Logf("  ImageSave()       : %v", imageSaveDuration)
	t.Logf("  untar             : %v", untarDuration)
	t.Logf("  full Save()       : %v", fullSaveDuration)
}

// buildConfigFile creates a minimal OCI config from the docker inspect response.
func buildConfigFile(inspect image.InspectResponse, layers []extractedLayer) map[string]interface{} {
	diffIDs := make([]string, len(layers))
	for i, l := range layers {
		diffIDs[i] = l.diffID
	}

	config := map[string]interface{}{
		"architecture": inspect.Architecture,
		"os":           inspect.Os,
		"rootfs": map[string]interface{}{
			"type":    "layers",
			"diff_ids": diffIDs,
		},
	}

	if inspect.Config != nil {
		imgConfig := map[string]interface{}{}
		if inspect.Config.Env != nil {
			imgConfig["Env"] = inspect.Config.Env
		}
		if inspect.Config.Cmd != nil {
			imgConfig["Cmd"] = inspect.Config.Cmd
		}
		if inspect.Config.Entrypoint != nil {
			imgConfig["Entrypoint"] = inspect.Config.Entrypoint
		}
		if inspect.Config.WorkingDir != "" {
			imgConfig["WorkingDir"] = inspect.Config.WorkingDir
		}
		if len(imgConfig) > 0 {
			config["config"] = imgConfig
		}
	}

	return config
}

func addToTar(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()
	hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header for %s: %v", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write tar data for %s: %v", name, err)
	}
}

func addBlobToTar(t *testing.T, tw *tar.Writer, digest string, data []byte) {
	t.Helper()
	// digest format is "sha256:hex", convert to "blobs/sha256/hex"
	parts := splitDigest(digest)
	name := fmt.Sprintf("blobs/%s/%s", parts[0], parts[1])
	addToTar(t, tw, name, data)
}

func splitDigest(digest string) [2]string {
	for i, c := range digest {
		if c == ':' {
			return [2]string{digest[:i], digest[i+1:]}
		}
	}
	return [2]string{"sha256", digest}
}

// TestSkopeoCopyCompressedLayerImage validates that images loaded with compressed
// layers in Docker legacy tar format can be correctly exported by skopeo.
// This is the exact validation that caught issue #300 -- images that passed
// ImageInspect but had malformed manifest/config layer relationships.
//
// Run with: go test -v -run TestSkopeoCopyCompressedLayerImage -count=1 ./local/
func TestSkopeoCopyCompressedLayerImage(t *testing.T) {
	if _, err := exec.LookPath("skopeo"); err != nil {
		t.Skip("skopeo not found in PATH, skipping")
	}

	docker := h.DockerCli(t)

	baseImageName := h.RunnableBaseImage("linux")
	h.PullIfMissing(t, docker, baseImageName)

	inspectResult, err := docker.ImageInspect(context.Background(), baseImageName)
	if err != nil {
		t.Fatalf("Failed to inspect base image: %v", err)
	}
	baseImageID := inspectResult.InspectResponse.ID

	// Extract base layers
	baseLayers := extractImageLayers(t, docker, baseImageID)

	// Build a multi-layer image (base + 3 added layers) to mimic a real build
	allLayers := make([]extractedLayer, len(baseLayers))
	copy(allLayers, baseLayers)

	for i := 0; i < 3; i++ {
		content := fmt.Sprintf("skopeo-test-layer-%d-%s", i, h.RandString(500))
		layerPath, err := h.CreateSingleFileLayerTar(
			fmt.Sprintf("/workspace/buildpack-%d/layer.txt", i),
			content, "linux",
		)
		if err != nil {
			t.Fatalf("create test layer: %v", err)
		}
		defer os.Remove(layerPath)

		layerData, err := os.ReadFile(layerPath)
		if err != nil {
			t.Fatalf("read test layer: %v", err)
		}

		diffID := fmt.Sprintf("sha256:%x", sha256.Sum256(layerData))

		var compressedBuf bytes.Buffer
		gzWriter := gzip.NewWriter(&compressedBuf)
		gzWriter.Write(layerData)
		gzWriter.Close()

		allLayers = append(allLayers, extractedLayer{
			diffID:           diffID,
			uncompressedData: layerData,
			compressedData:   compressedBuf.Bytes(),
			uncompressedSize: int64(len(layerData)),
			compressedSize:   int64(compressedBuf.Len()),
		})
	}

	// Test both compressed and uncompressed images with skopeo
	t.Run("Uncompressed", func(t *testing.T) {
		repoName := "imgutil-skopeo-test-uncomp-" + h.RandString(10) + ":latest"
		defer h.DockerRmi(docker, repoName)

		tarData := buildDockerTar(t, allLayers, inspectResult.InspectResponse, repoName, false)
		loadImage(t, docker, tarData)

		runSkopeoCopy(t, repoName, "uncompressed")
	})

	t.Run("Compressed", func(t *testing.T) {
		repoName := "imgutil-skopeo-test-comp-" + h.RandString(10) + ":latest"
		defer h.DockerRmi(docker, repoName)

		tarData := buildDockerTar(t, allLayers, inspectResult.InspectResponse, repoName, true)
		loadImage(t, docker, tarData)

		runSkopeoCopy(t, repoName, "compressed")
	})

	// Also test the current imgutil Save() path for comparison
	t.Run("ImgutilSave", func(t *testing.T) {
		repoName := "imgutil-skopeo-test-imgutil-" + h.RandString(10)
		defer h.DockerRmi(docker, repoName)

		img, err := local.NewImage(repoName, docker, local.FromBaseImage(baseImageName))
		if err != nil {
			t.Fatalf("NewImage: %v", err)
		}

		for i := 0; i < 3; i++ {
			layerPath, err := h.CreateSingleFileLayerTar(
				fmt.Sprintf("/workspace/buildpack-%d/layer.txt", i),
				fmt.Sprintf("imgutil-test-layer-%d-%s", i, h.RandString(500)),
				"linux",
			)
			if err != nil {
				t.Fatalf("create layer: %v", err)
			}
			defer os.Remove(layerPath)
			if err := img.AddLayer(layerPath); err != nil {
				t.Fatalf("AddLayer: %v", err)
			}
		}

		if err := img.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}

		runSkopeoCopy(t, repoName+":latest", "imgutil-save")
	})
}

func loadImage(t *testing.T, docker client.APIClient, tarData []byte) {
	t.Helper()
	loadResult, err := docker.ImageLoad(context.Background(), bytes.NewReader(tarData), client.ImageLoadWithQuiet(true))
	if err != nil {
		t.Fatalf("ImageLoad failed: %v", err)
	}
	resp, _ := io.ReadAll(loadResult)
	loadResult.Close()
	t.Logf("ImageLoad response: %s", string(resp))
}

func runSkopeoCopy(t *testing.T, imageName, label string) {
	t.Helper()

	ociDir, err := os.MkdirTemp("", "imgutil-skopeo-oci-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(ociDir)

	src := fmt.Sprintf("docker-daemon:%s", imageName)
	dst := fmt.Sprintf("oci:%s", ociDir)

	t.Logf("SKOPEO [%s]: copying %s -> %s", label, src, dst)

	cmd := exec.Command("skopeo", "copy", src, dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("SKOPEO [%s]: FAILED\n  command: skopeo copy %s %s\n  error: %v\n  output: %s",
			label, src, dst, err, string(output))
		return
	}
	t.Logf("SKOPEO [%s]: SUCCESS\n  output: %s", label, string(output))

	// Verify the OCI layout is valid by reading index.json
	indexData, err := os.ReadFile(filepath.Join(ociDir, "index.json"))
	if err != nil {
		t.Errorf("SKOPEO [%s]: OCI layout missing index.json: %v", label, err)
		return
	}

	var index struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(indexData, &index); err != nil {
		t.Errorf("SKOPEO [%s]: invalid index.json: %v", label, err)
		return
	}

	if len(index.Manifests) == 0 {
		t.Errorf("SKOPEO [%s]: index.json has no manifests", label)
		return
	}

	// Read and validate the manifest
	manifestDigest := index.Manifests[0].Digest
	parts := splitDigest(manifestDigest)
	manifestPath := filepath.Join(ociDir, "blobs", parts[0], parts[1])
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Errorf("SKOPEO [%s]: cannot read manifest blob: %v", label, err)
		return
	}

	var manifest struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			Digest    string `json:"digest"`
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Errorf("SKOPEO [%s]: invalid manifest: %v", label, err)
		return
	}

	// Read the config and verify diff_ids match layers
	configParts := splitDigest(manifest.Config.Digest)
	configPath := filepath.Join(ociDir, "blobs", configParts[0], configParts[1])
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("SKOPEO [%s]: cannot read config blob: %v", label, err)
		return
	}

	var config struct {
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Errorf("SKOPEO [%s]: invalid config: %v", label, err)
		return
	}

	t.Logf("SKOPEO [%s]: manifest has %d layers, config has %d diff_ids",
		label, len(manifest.Layers), len(config.RootFS.DiffIDs))

	if len(manifest.Layers) != len(config.RootFS.DiffIDs) {
		t.Errorf("SKOPEO [%s]: MISMATCH - manifest layers (%d) != config diff_ids (%d)",
			label, len(manifest.Layers), len(config.RootFS.DiffIDs))
		return
	}

	// Verify each layer blob exists and has correct size
	for i, layer := range manifest.Layers {
		layerParts := splitDigest(layer.Digest)
		layerPath := filepath.Join(ociDir, "blobs", layerParts[0], layerParts[1])
		fi, err := os.Stat(layerPath)
		if err != nil {
			t.Errorf("SKOPEO [%s]: layer %d blob missing: %v", label, i, err)
			continue
		}
		if fi.Size() != layer.Size {
			t.Errorf("SKOPEO [%s]: layer %d size mismatch: manifest says %d, file is %d",
				label, i, layer.Size, fi.Size())
			continue
		}

		// Verify the blob content matches the digest
		blobData, err := os.ReadFile(layerPath)
		if err != nil {
			t.Errorf("SKOPEO [%s]: cannot read layer %d blob: %v", label, i, err)
			continue
		}
		actualDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(blobData))
		if actualDigest != layer.Digest {
			t.Errorf("SKOPEO [%s]: layer %d digest mismatch: manifest says %s, actual %s",
				label, i, layer.Digest, actualDigest)
		}
	}

	t.Logf("SKOPEO [%s]: all layer blobs verified", label)
}
