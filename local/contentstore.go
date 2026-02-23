package local

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	contentapi "github.com/containerd/containerd/api/services/content/v1"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	v1types "github.com/google/go-containerregistry/pkg/v1/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"
)

// contentStoreLayer implements v1.Layer by reading blob data from
// the containerd content store via gRPC. This avoids the expensive
// ImageSave/untar/decompress pipeline used by doDownloadLayersFor.
type contentStoreLayer struct {
	diffID         v1.Hash
	compressedDgst string
	compressedSize int64
	isCompressed   bool // true if layer is gzip-compressed in the content store
	contentClient  contentapi.ContentClient
}

func (l *contentStoreLayer) DiffID() (v1.Hash, error) {
	return l.diffID, nil
}

func (l *contentStoreLayer) Digest() (v1.Hash, error) {
	return v1.Hash{}, nil
}

func (l *contentStoreLayer) Uncompressed() (io.ReadCloser, error) {
	reader, err := readBlobStream(context.Background(), l.contentClient, l.compressedDgst)
	if err != nil {
		return nil, fmt.Errorf("reading blob %s: %w", l.compressedDgst[:20], err)
	}
	if !l.isCompressed {
		return reader, nil
	}
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("decompressing blob %s: %w", l.compressedDgst[:20], err)
	}
	return &gzipReadCloser{gzReader: gzReader, underlying: reader}, nil
}

func (l *contentStoreLayer) Compressed() (io.ReadCloser, error) {
	return readBlobStream(context.Background(), l.contentClient, l.compressedDgst)
}

func (l *contentStoreLayer) Size() (int64, error) {
	// Return compressed size. This must be > 0 to indicate the layer has data
	// (Size() == -1 is used as a sentinel for "empty/blank layer" in addLayerToTar).
	return l.compressedSize, nil
}

func (l *contentStoreLayer) MediaType() (v1types.MediaType, error) {
	return v1types.DockerLayer, nil
}

// bufferedConn wraps a net.Conn so that reads drain any buffered data
// from a bufio.Reader before reading from the underlying connection.
// This is needed after HTTP upgrade: the bufio.Reader used to parse the
// HTTP response may have read ahead into the h2c stream.
type bufferedConn struct {
	net.Conn
	br *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.br.Read(p)
}

// gzipReadCloser closes both the gzip reader and the underlying reader.
type gzipReadCloser struct {
	gzReader   *gzip.Reader
	underlying io.ReadCloser
}

func (r *gzipReadCloser) Read(p []byte) (int, error) {
	return r.gzReader.Read(p)
}

func (r *gzipReadCloser) Close() error {
	r.gzReader.Close()
	return r.underlying.Close()
}

// doDownloadLayersViaContentStore reads layer metadata from the containerd
// content store via gRPC, avoiding the expensive ImageSave/untar/decompress
// pipeline. Each base layer is decompressed 0 times during this phase
// (vs 1 time in the current doDownloadLayersFor for diffID computation).
func (s *Store) doDownloadLayersViaContentStore(identifier string) error {
	if identifier == "" {
		return nil
	}
	ctx := context.Background()

	conn, err := dialDockerGRPC()
	if err != nil {
		return fmt.Errorf("connecting to content store: %w", err)
	}
	// Store connection so it stays alive while layers reference the client
	s.grpcConn = conn

	contentClient := contentapi.NewContentClient(conn)

	// Step 1: Read the image descriptor blob.
	// The identifier from ImageInspect is the image's content-addressable digest.
	// With containerd, this may be a manifest list or a single manifest.
	// We need to resolve through to find the actual image manifest.
	manifestDigest, err := resolveImageManifest(ctx, contentClient, identifier)
	if err != nil {
		return fmt.Errorf("resolving manifest for %s: %w", identifier[:20], err)
	}

	// Step 2: Read the manifest
	manifestBlob, err := readBlob(ctx, contentClient, manifestDigest)
	if err != nil {
		return fmt.Errorf("reading manifest %s: %w", manifestDigest[:20], err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBlob, &manifest); err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	// Step 3: Read the config to get diff_ids
	configDigest := manifest.Config.Digest.String()
	configBlob, err := readBlob(ctx, contentClient, configDigest)
	if err != nil {
		return fmt.Errorf("reading config %s: %w", configDigest[:20], err)
	}

	var configFile struct {
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err := json.Unmarshal(configBlob, &configFile); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	if len(configFile.RootFS.DiffIDs) != len(manifest.Layers) {
		return fmt.Errorf("layer count mismatch: config has %d diff_ids, manifest has %d layers",
			len(configFile.RootFS.DiffIDs), len(manifest.Layers))
	}

	// Step 4: Create contentStoreLayer for each layer and populate the store
	for i, layerDesc := range manifest.Layers {
		diffIDStr := configFile.RootFS.DiffIDs[i]
		diffID, err := v1.NewHash(diffIDStr)
		if err != nil {
			return fmt.Errorf("parsing diff_id %s: %w", diffIDStr, err)
		}

		compressedDigest := layerDesc.Digest.String()
		mediaType := string(layerDesc.MediaType)
		isCompressed := strings.Contains(mediaType, "gzip") || strings.Contains(mediaType, "+gzip")
		layer := &contentStoreLayer{
			diffID:         diffID,
			compressedDgst: compressedDigest,
			compressedSize: layerDesc.Size,
			isCompressed:   isCompressed,
			contentClient:  contentClient,
		}

		s.onDiskLayersByDiffID[diffID] = annotatedLayer{
			layer:            layer,
			uncompressedSize: -1, // computed during write via temp file
		}
	}

	debugLog("[imgutil] doDownloadLayersViaContentStore: loaded %d layers from content store (0 decompressions)",
		len(manifest.Layers))
	return nil
}

// resolveImageManifest resolves an image identifier (which may be a manifest list)
// to the platform-specific manifest digest.
func resolveImageManifest(ctx context.Context, client contentapi.ContentClient, identifier string) (string, error) {
	blob, err := readBlob(ctx, client, identifier)
	if err != nil {
		return "", err
	}

	// Try parsing as an OCI index (manifest list)
	var index ocispec.Index
	if err := json.Unmarshal(blob, &index); err == nil && len(index.Manifests) > 0 {
		// It's a manifest list. Find a platform-specific manifest that
		// actually exists in the content store. Docker only pulls the
		// platform variant it needs, so most sub-manifests won't be present.
		for _, m := range index.Manifests {
			// Skip attestation manifests
			if m.Platform != nil && m.Platform.Architecture == "unknown" {
				continue
			}
			if blobExists(ctx, client, m.Digest.String()) {
				return m.Digest.String(), nil
			}
		}
		return "", fmt.Errorf("no manifest from index found in content store")
	}

	// Try parsing as a single manifest
	var manifest ocispec.Manifest
	if err := json.Unmarshal(blob, &manifest); err == nil && manifest.Config.Digest != "" {
		return identifier, nil // Already a manifest
	}

	return "", fmt.Errorf("blob %s is neither a manifest nor an index", identifier[:20])
}

// readBlob reads an entire blob from the content store.
func readBlob(ctx context.Context, client contentapi.ContentClient, digest string) ([]byte, error) {
	stream, err := client.Read(ctx, &contentapi.ReadContentRequest{
		Digest: digest,
	})
	if err != nil {
		return nil, err
	}

	var data []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		data = append(data, resp.Data...)
	}
	return data, nil
}

// readBlobStream returns a streaming reader for a blob from the content store.
func readBlobStream(ctx context.Context, client contentapi.ContentClient, digest string) (io.ReadCloser, error) {
	stream, err := client.Read(ctx, &contentapi.ReadContentRequest{
		Digest: digest,
	})
	if err != nil {
		return nil, err
	}
	return &grpcBlobReader{stream: stream}, nil
}

// grpcBlobReader adapts a gRPC Read stream to io.ReadCloser.
type grpcBlobReader struct {
	stream contentapi.Content_ReadClient
	buf    []byte
}

func (r *grpcBlobReader) Read(p []byte) (int, error) {
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}
	resp, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}
	n := copy(p, resp.Data)
	if n < len(resp.Data) {
		r.buf = resp.Data[n:]
	}
	return n, nil
}

func (r *grpcBlobReader) Close() error {
	return nil
}

// dialDockerGRPC connects to Docker's gRPC endpoint via the /grpc HTTP upgrade path.
func dialDockerGRPC() (*grpc.ClientConn, error) {
	sockPath := dockerSocketPath()
	conn, err := grpc.NewClient(
		"passthrough:///docker",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			dialer := net.Dialer{}
			conn, err := dialer.DialContext(ctx, "unix", sockPath)
			if err != nil {
				return nil, err
			}

			req := "POST /grpc HTTP/1.1\r\n" +
				"Host: docker\r\n" +
				"Content-Type: application/grpc\r\n" +
				"Connection: Upgrade\r\n" +
				"Upgrade: h2c\r\n" +
				"\r\n"
			if _, err := conn.Write([]byte(req)); err != nil {
				conn.Close()
				return nil, fmt.Errorf("writing upgrade request: %w", err)
			}

			// Read the HTTP response. The bufio.Reader may read ahead
			// into the h2c stream, so we return a wrapped connection
			// that drains the buffer before reading from the raw conn.
			br := bufio.NewReader(conn)
			resp, err := http.ReadResponse(br, nil)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("reading upgrade response: %w", err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusSwitchingProtocols {
				conn.Close()
				return nil, fmt.Errorf("expected 101 Switching Protocols, got %d %s", resp.StatusCode, resp.Status)
			}

			return &bufferedConn{Conn: conn, br: br}, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	// Quick probe: verify the content store is accessible
	contentClient := contentapi.NewContentClient(conn)
	listStream, err := contentClient.List(context.Background(), &contentapi.ListContentRequest{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("content store not accessible: %w", err)
	}
	// Read and discard one response to confirm connectivity
	_, err = listStream.Recv()
	if err != nil && err != io.EOF {
		conn.Close()
		return nil, fmt.Errorf("content store probe failed: %w", err)
	}

	return conn, nil
}

// dockerSocketPath returns the path to the Docker socket.
func dockerSocketPath() string {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return strings.TrimPrefix(host, "unix://")
	}
	home, _ := os.UserHomeDir()
	ddSocket := home + "/.docker/run/docker.sock"
	if _, err := os.Stat(ddSocket); err == nil {
		return ddSocket
	}
	return "/var/run/docker.sock"
}

// blobExists checks if a blob exists in the content store.
func blobExists(ctx context.Context, client contentapi.ContentClient, digest string) bool {
	_, err := client.Info(ctx, &contentapi.InfoRequest{Digest: digest})
	if err == nil {
		return true
	}
	if s, ok := grpcstatus.FromError(err); ok && s.Code() == codes.NotFound {
		return false
	}
	return false
}
