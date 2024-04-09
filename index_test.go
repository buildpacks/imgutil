package imgutil_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	"github.com/buildpacks/imgutil/index"
	"github.com/buildpacks/imgutil/layout"
	"github.com/buildpacks/imgutil/local"
	"github.com/buildpacks/imgutil/remote"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestIndex(t *testing.T) {
	spec.Run(t, "Index", testIndex, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testIndex(t *testing.T, when spec.G, it spec.S) {
	var (
		xdgPath string
		err     error
	)

	it.Before(func() {
		// creates the directory to save all the OCI images on disk
		xdgPath, err = os.MkdirTemp("", "image-indexes")
		h.AssertNil(t, err)
	})

	it.After(func() {
		err := os.RemoveAll(xdgPath)
		h.AssertNil(t, err)
	})

	when("#ManifestHandler", func() {
		when("#OS", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				_, err := idx.OS(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #OS requested", func() {
				digest, err := name.NewDigest("busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				os, err := idx.OS(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, os, "")
			})
			it("should return latest OS when os of the given digest annotated", func() {
				digest, err := name.NewDigest("busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							hash: {
								Platform: &v1.Platform{
									OS: "some-os",
								},
							},
						},
					},
				}

				os, err := idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
			it("should return an error when an image with the given digest doesn't exists", func() {
				digest, err := name.NewDigest("busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				os, err := idx.OS(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, os, "")
			})
			it("should return expected os when os is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, v1.ImageIndex(nil))

				os, err := idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")
			})
		})
		when("#SetOS", func() {
			it("should return an error when invalid digest is provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				err := idx.SetOS(digest, "some-os")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetOS requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetOS(digest, "some-os")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should SetOS for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.SetOS(digest, "some-os")
				h.AssertNil(t, err)

				os, err := idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.SetOS(digest, "some-os")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("#Architecture", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				_, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #Architecture requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				os, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, os, "")
			})
			it("should return latest Architecture when arch of the given digest annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							hash: {
								Platform: &v1.Platform{
									Architecture: "some-arch",
								},
							},
						},
					},
				}

				arch, err := idx.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "some-arch")
			})
			it("should return an error when an image with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				arch, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, arch, "")
			})
			it("should return expected Architecture when arch is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, v1.ImageIndex(nil))

				arch, err := idx.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm")
			})
		})
		when("#SetArchitecture", func() {
			it("should return an error when invalid digest is provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				err := idx.SetArchitecture(digest, "some-arch")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetArchitecture requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetArchitecture(digest, "some-arch")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should SetArchitecture for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.SetArchitecture(digest, "some-arch")
				h.AssertNil(t, err)

				os, err := idx.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-arch")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.SetArchitecture(digest, "some-arch")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("#Variant", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				_, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #Variant requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				variant, err := idx.Variant(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, variant, "")
			})
			it("should return latest Variant when variant of the given digest annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							hash: {
								Platform: &v1.Platform{
									Variant: "some-variant",
								},
							},
						},
					},
				}

				variant, err := idx.Variant(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "some-variant")
			})
			it("should return an error when an image with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				arch, err := idx.Variant(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, arch, "")
			})
			it("should return expected Variant when arch is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, v1.ImageIndex(nil))

				arch, err := idx.Variant(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "v6")
			})
		})
		when("#SetVariant", func() {
			it("should return an error when invalid digest is provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				err := idx.SetVariant(digest, "some-variant")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetVariant requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetVariant(digest, "some-variant")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should SetVariant for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.SetVariant(digest, "some-variant")
				h.AssertNil(t, err)

				os, err := idx.Variant(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-variant")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.SetVariant(digest, "some-variant")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("#OSVersion", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				_, err := idx.OSVersion(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #OSVersion requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				osVersion, err := idx.OSVersion(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, osVersion, "")
			})
			it("should return latest OSVersion when osVersion of the given digest annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							hash: {
								Platform: &v1.Platform{
									OSVersion: "some-osVersion",
								},
							},
						},
					},
				}

				variant, err := idx.OSVersion(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "some-osVersion")
			})
			it("should return an error when an image with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				osVersion, err := idx.OSVersion(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, osVersion, "")
			})
			it("should return expected OSVersion when arch is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, v1.ImageIndex(nil))

				err = idx.SetOSVersion(digest, "some-osVersion")
				h.AssertNil(t, err)

				osVersion, err := idx.OSVersion(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, osVersion, "some-osVersion")
			})
		})
		when("#SetOSVersion", func() {
			it("should return an error when invalid digest is provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				err := idx.SetOSVersion(digest, "some-osVersion")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetOSVersion requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetOSVersion(digest, "some-osVersion")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should SetOSVersion for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.SetOSVersion(digest, "some-osVersion")
				h.AssertNil(t, err)

				os, err := idx.OSVersion(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-osVersion")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.SetOSVersion(digest, "some-osVersion")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("#Features", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				_, err := idx.Features(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #Features is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				features, err := idx.Features(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))
			})
			it("should return annotated Features when Features of the image/index is annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							hash: {
								Platform: &v1.Platform{
									Features: []string{"some-features"},
								},
							},
						},
					},
				}

				features, err := idx.Features(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, features, []string{"some-features"})
			})
			it("should return error if the image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				features, err := idx.Features(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))
			})
			it("should return expected Features of the given image/index when image/index is not annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, v1.ImageIndex(nil))

				err = idx.SetFeatures(digest, []string{"some-features"})
				h.AssertNil(t, err)

				features, err := idx.Features(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, features, []string{"some-features"})
			})
		})
		when("#SetFeatures", func() {
			it("should return an error when an invalid digest is provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				err := idx.SetFeatures(digest, []string{"some-features"})
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #SetFeatures is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetFeatures(digest, []string{"some-features"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should SetFeatures when the given digest is image/index", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.SetFeatures(digest, []string{"some-features"})
				h.AssertNil(t, err)

				features, err := idx.Features(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, features, []string{"some-features"})
			})
			it("should return an error when no image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.SetFeatures(digest, []string{"some-features"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("#OSFeatures", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				_, err := idx.OSFeatures(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #OSFeatures is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				osFeatures, err := idx.OSFeatures(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))
			})
			it("should return annotated OSFeatures when OSFeatures of the image/index is annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							hash: {
								Platform: &v1.Platform{
									OSFeatures: []string{"some-osFeatures"},
								},
							},
						},
					},
				}

				osFeatures, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{"some-osFeatures"})
			})
			it("should return the OSFeatures if the image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				osFeatures, err := idx.OSFeatures(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))
			})
			it("should return expected OSFeatures of the given image when image/index is not annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, v1.ImageIndex(nil))

				err = idx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertNil(t, err)

				osFeatures, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{"some-osFeatures"})
			})
		})
		when("#SetOSFeatures", func() {
			it("should return an error when an invalid digest is provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				err := idx.SetFeatures(digest, []string{"some-osFeatures"})
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #SetOSFeatures is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should SetOSFeatures when the given digest is image/index", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertNil(t, err)

				osFeatures, err := idx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{"some-osFeatures"})
			})
			it("should return an error when no image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("docker manifest list", func() {
			when("#Annotations", func() {
				it("should return an error when invalid digest provided", func() {
					digest := name.Digest{}
					idx := imgutil.ManifestHandler{}
					_, err := idx.OSFeatures(digest)
					h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
				})
				it("should return an error when a removed manifest's #Annotations is requested", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					hash, err := v1.NewHash(digest.Identifier())
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: imgutil.NewEmptyDockerIndex(),
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return annotated Annotations when Annotations of the image/index is annotated", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx, err := remote.NewIndex(
						"alpine:3.19.0",
						index.WithInsecure(true),
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath(xdgPath),
					)
					h.AssertNil(t, err)

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return the Annotations if the image/index with the given digest exists", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: imgutil.NewEmptyDockerIndex(),
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return expected Annotations of the given image/index when image/index is not annotated", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx, err := remote.NewIndex("alpine:3.19.0", index.WithXDGRuntimePath(xdgPath))
					h.AssertNil(t, err)
					h.AssertNotEq(t, idx, v1.ImageIndex(nil))

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
			})
			when("#SetAnnotations", func() {
				it("should return an error when invalid digest provided", func() {
					digest := name.Digest{}
					idx := imgutil.ManifestHandler{}
					err := idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
				})
				it("should return an error if the image/index is removed", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					hash, err := v1.NewHash(digest.Identifier())
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: imgutil.NewEmptyDockerIndex(),
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				})
				it("should SetAnnotations when an image/index with the given digest exists", func() {
					idx, err := remote.NewIndex(
						"alpine:latest",
						index.WithInsecure(true),
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath(xdgPath),
					)
					h.AssertNil(t, err)

					imgIdx, ok := idx.(*imgutil.ManifestHandler)
					h.AssertEq(t, ok, true)

					mfest, err := imgIdx.ImageIndex.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("alpine@" + hash.String())
					h.AssertNil(t, err)

					err = imgIdx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := imgIdx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return an error if the manifest with the given digest is neither image nor index", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: imgutil.NewEmptyDockerIndex(),
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				})
			})
		})
		when("oci image index", func() {
			when("#Annotations", func() {
				it("should return an error when invalid digest provided", func() {
					digest := name.Digest{}
					idx := imgutil.ManifestHandler{}
					_, err := idx.OSFeatures(digest)
					h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
				})
				it("should return an error when a removed manifest's #Annotations is requested", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					hash, err := v1.NewHash(digest.Identifier())
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: empty.Index,
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return annotated Annotations when Annotations of the image/index is annotated", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx, err := remote.NewIndex(
						"busybox:1.36-musl",
						index.WithInsecure(true),
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath(xdgPath),
					)
					h.AssertNil(t, err)

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertNil(t, err)
					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should return the Annotations if the image/index with the given digest exists", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: empty.Index,
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return expected Annotations of the given image when image/index is not annotated", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath(xdgPath))
					h.AssertNil(t, err)
					h.AssertNotEq(t, idx, v1.ImageIndex(nil))

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertNil(t, err)
					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
			})
			when("#SetAnnotations", func() {
				it("should return an error when invalid digest provided", func() {
					digest := name.Digest{}
					idx := imgutil.ManifestHandler{}
					err := idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
				})
				it("should return an error if the image/index is removed", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					hash, err := v1.NewHash(digest.Identifier())
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: empty.Index,
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				})
				it("should SetAnnotations when an image/index with the given digest exists", func() {
					idx, err := remote.NewIndex(
						"busybox:1.36-musl",
						index.WithInsecure(true),
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath(xdgPath),
					)
					h.AssertNil(t, err)

					digest, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertNil(t, err)
					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should return an error if the manifest with the given digest is neither image nor index", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.ManifestHandler{
						ImageIndex: empty.Index,
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				})
			})
		})
		when("#URLs", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				_, err := idx.URLs(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #URLs is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				urls, err := idx.URLs(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))
			})
			it("should return annotated URLs when URLs of the image/index is annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							hash: {
								URLs: []string{
									"some-urls",
								},
							},
						},
					},
				}

				urls, err := idx.URLs(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, urls, []string{
					"some-urls",
				})
			})
			it("should return the URLs if the image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				urls, err := idx.URLs(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))
			})
			it("should return expected URLs of the given image when image/index is not annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)
				h.AssertNotEq(t, idx, v1.ImageIndex(nil))

				err = idx.SetURLs(digest, []string{
					"some-urls",
				})
				h.AssertNil(t, err)

				urls, err := idx.URLs(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, urls, []string{
					"some-urls",
				})
			})
		})
		when("#SetURLs", func() {
			it("should return an error when an invalid digest is provided", func() {
				digest := name.Digest{}
				idx := imgutil.ManifestHandler{}
				err := idx.SetURLs(digest, []string{"some-urls"})
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #SetURLs is requested", func() {
				digest, err := name.NewDigest("busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetURLs(digest, []string{
					"some-urls",
				})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should SetOSFeatures when the given digest is image/index", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.SetURLs(digest, []string{
					"some-urls",
				})
				h.AssertNil(t, err)

				urls, err := idx.URLs(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, urls, []string{
					"some-urls",
				})
			})
			it("should return an error when no image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.SetURLs(digest, []string{
					"some-urls",
				})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("#Add", func() {
			it("should return an error when the image/index with the given reference doesn't exists", func() {
				_, err := remote.NewIndex(
					"unknown/index",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertEq(t, err.Error(), "GET https://index.docker.io/v2/unknown/index/manifests/latest: UNAUTHORIZED: authentication required; [map[Action:pull Class: Name:unknown/index Type:repository]]")
			})
			when("platform specific", func() {
				it("should add platform specific image", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					idx, err := layout.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:3.19",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithOS("linux"),
						imgutil.WithArchitecture("amd64"),
					)
					h.AssertNil(t, err)

					index := idx.(*imgutil.ManifestHandler)

					hashes := make([]v1.Hash, 0, len(index.Images))
					for h2 := range index.Images {
						hashes = append(hashes, h2)
					}
					h.AssertEq(t, len(hashes), 1)

					digest, err := name.NewDigest("alpine@sha256:6457d53fb065d6f250e1504b9bc42d5b6c65941d57532c072d929dd0628977d0", name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should add annotations when WithAnnotations used for oci", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					idx, err := layout.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"busybox:1.36-musl",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithOS("linux"),
						imgutil.WithArchitecture("amd64"),
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
					)
					h.AssertNil(t, err)

					digest, err := name.NewDigest("busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34", name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertNil(t, err)

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should not add annotations when WithAnnotations used for docker", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					idx, err := local.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:3.19.0",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithOS("linux"),
						imgutil.WithArchitecture("amd64"),
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
					)
					h.AssertNil(t, err)

					digest, err := name.NewDigest("alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd", name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
			})
			when("target specific", func() {
				it("should add target specific image", func() {
					if runtime.GOOS == "windows" {
						// TODO we need to prepare a registry image for windows
						t.Skip("alpine is not available for windows")
					}

					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					idx, err := layout.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:3.19.0",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(ref)
					h.AssertNil(t, err)

					index := idx.(*imgutil.ManifestHandler)
					hashes := make([]v1.Hash, 0, len(index.Images))
					for h2 := range index.Images {
						hashes = append(hashes, h2)
					}
					h.AssertEq(t, len(hashes), 1)

					hash := hashes[0]
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, runtime.GOOS)

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, runtime.GOARCH)

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should add annotations when WithAnnotations used for oci", func() {
					if runtime.GOOS == "windows" {
						// TODO we need to prepare a registry image for windows
						t.Skip("busybox is not available for windows")
					}

					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					idx, err := layout.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"busybox:1.36-musl",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
					)
					h.AssertNil(t, err)

					index := idx.(*imgutil.ManifestHandler)
					hashes := make([]v1.Hash, 0, len(index.Images))
					for h2 := range index.Images {
						hashes = append(hashes, h2)
					}
					h.AssertEq(t, len(hashes), 1)

					hash := hashes[0]
					digest, err := name.NewDigest("busybox@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, runtime.GOOS)

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, runtime.GOARCH)

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.OCIImageIndex, digest.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertNil(t, err)

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should not add annotations when WithAnnotations used for docker", func() {
					if runtime.GOOS == "windows" {
						// TODO we need to prepare a registry image for windows
						t.Skip("alpine is not available for windows")
					}

					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					idx, err := local.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:latest",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
					)
					h.AssertNil(t, err)

					index := idx.(*imgutil.ManifestHandler)
					hashes := make([]v1.Hash, 0, len(index.Images))
					for h2 := range index.Images {
						hashes = append(hashes, h2)
					}
					h.AssertEq(t, len(hashes), 1)

					hash := hashes[0]
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, runtime.GOOS)

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, runtime.GOARCH)

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
			})
			when("image specific", func() {
				it("should not change the digest of the image when added", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					idx, err := layout.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(ref)
					h.AssertNil(t, err)

					index := idx.(*imgutil.ManifestHandler)
					hashes := make([]v1.Hash, 0, len(index.Images))
					for h2 := range index.Images {
						hashes = append(hashes, h2)
					}

					h.AssertEq(t, len(hashes), 1)
					hash := hashes[0]
					digest, err := name.NewDigest(
						"alpine@"+hash.String(),
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should annotate the annotations when Annotations provided for oci", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					idx, err := layout.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath))
					h.AssertNil(t, err)

					index := idx.(*imgutil.ManifestHandler)
					ref, err := name.ParseReference(
						"busybox@sha256:648143a312f16e5b5a6f64dfa4024a281fb4a30467500ca8b0091a9984f1c751",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = index.Add(
						ref,
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
					)
					h.AssertNil(t, err)

					hashes := make([]v1.Hash, 0, len(index.Images))
					for h2 := range index.Images {
						hashes = append(hashes, h2)
					}

					h.AssertEq(t, len(hashes), 1)
					hash := hashes[0]
					digest, err := name.NewDigest("busybox@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "arm64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err, nil)
					h.AssertEq(t, variant, "v8")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertNil(t, err)

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should not annotate the annotations when Annotations provided for docker", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					idx, err := local.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
					)
					h.AssertNil(t, err)

					index := idx.(*imgutil.ManifestHandler)
					hashes := make([]v1.Hash, 0, len(index.Images))
					for h2 := range index.Images {
						hashes = append(hashes, h2)
					}
					h.AssertEq(t, len(hashes), 1)

					hash := hashes[0]
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest.Identifier()).Error())

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, false)
					h.AssertEq(t, v, "")
				})
			})
			when("index specific", func() {
				it("should add all the images of the given reference", func() {
					_, err := index.NewIndex(
						"some/image:tag",
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath(xdgPath),
						index.WithFormat(types.DockerManifestList),
					)
					h.AssertNil(t, err)

					idx, err := local.NewIndex(
						"some/image:tag",
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath(xdgPath),
					)
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:3.19.0",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					// linux/amd64
					digest1, err := name.NewDigest(
						"alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					// linux arm/v6
					digest2, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(ref, imgutil.WithAll(true))
					h.AssertNil(t, err)

					os, err := idx.OS(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := idx.Architecture(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := idx.Variant(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := idx.OSVersion(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := idx.Features(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := idx.OSFeatures(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := idx.URLs(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := idx.Annotations(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))

					os, err = idx.OS(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err = idx.Architecture(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "arm")

					variant, err = idx.Variant(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, variant, "v6")

					osVersion, err = idx.OSVersion(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest2.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err = idx.Features(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err = idx.OSFeatures(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err = idx.URLs(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err = idx.Annotations(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest2.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should not ignore WithAnnotations for oci", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					idx, err := layout.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"busybox:1.36-musl",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					digest1, err := name.NewDigest(
						"busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					digest2, err := name.NewDigest(
						"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
						imgutil.WithAll(true),
					)
					h.AssertNil(t, err)

					os, err := idx.OS(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := idx.Architecture(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := idx.Variant(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := idx.OSVersion(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := idx.Features(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := idx.OSFeatures(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := idx.URLs(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := idx.Annotations(digest1)
					h.AssertNil(t, err)

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")

					os, err = idx.OS(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err = idx.Architecture(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "arm")

					arch, err = idx.Variant(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "v6")

					osVersion, err = idx.OSVersion(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err = idx.Features(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err = idx.OSFeatures(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err = idx.URLs(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err = idx.Annotations(digest2)
					h.AssertNil(t, err)

					v, ok = annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should ignore WithAnnotations for docker", func() {
					_, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					idx, err := local.NewIndex("some/image:tag", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.DockerManifestList))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:3.19.0",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					digest1, err := name.NewDigest(
						"alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					digest2, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(
						ref,
						imgutil.WithAnnotations(map[string]string{
							"some-key": "some-value",
						}),
						imgutil.WithAll(true),
					)
					h.AssertNil(t, err)

					os, err := idx.OS(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := idx.Architecture(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := idx.Variant(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, variant, "")

					osVersion, err := idx.OSVersion(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err := idx.Features(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := idx.OSFeatures(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := idx.URLs(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := idx.Annotations(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))

					os, err = idx.OS(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err = idx.Architecture(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "arm")

					variant, err = idx.Variant(digest2)
					h.AssertNil(t, err)
					h.AssertEq(t, variant, "v6")

					osVersion, err = idx.OSVersion(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest2.Identifier()).Error())
					h.AssertEq(t, osVersion, "")

					features, err = idx.Features(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err = idx.OSFeatures(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err = idx.URLs(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err = idx.Annotations(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest1.Identifier()).Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
			})
		})
		when("#Save", func() {
			it("should save the index", func() {
				idx, err := remote.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				_, err = local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)
			})
			it("should save the annotated index", func() {
				idx, err := remote.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest("some/index@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd")
				h.AssertNil(t, err)

				err = idx.SetOS(digest, "some-os")
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				// locally saved image should also work as expected
				indx, err := local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err := indx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")

				err = indx.SetOS(digest, "some-os")
				h.AssertNil(t, err)

				err = indx.SetArchitecture(digest, "something")
				h.AssertNil(t, err)

				err = indx.SetVariant(digest, "something")
				h.AssertNil(t, err)

				err = indx.SetOSVersion(digest, "something")
				h.AssertNil(t, err)

				err = indx.SetFeatures(digest, []string{"some-features"})
				h.AssertNil(t, err)

				err = indx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertNil(t, err)

				err = indx.SetURLs(digest, []string{"some-urls"})
				h.AssertNil(t, err)

				err = indx.SetAnnotations(digest, map[string]string{"some-key": "some-value"})
				h.AssertNil(t, err)

				h.AssertNil(t, indx.Save())

				idx, err = local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err = idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
			it("should save the added yet annotated images", func() {
				idx, err := remote.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				digest, err := name.NewDigest("busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34", name.WeakValidation)
				h.AssertNil(t, err)

				err = idx.Add(digest)
				h.AssertNil(t, err)

				err = idx.SetOS(digest, "some-os")
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err := idx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
			it("should save all added images", func() {
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx1, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox:1.36-musl", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx1.Add(ref, imgutil.WithAll(true))
				h.AssertNil(t, err)

				ii1, ok := idx1.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				hashes := make([]v1.Hash, 0, len(ii1.Images))
				for h2 := range ii1.Images {
					hashes = append(hashes, h2)
				}
				h.AssertEq(t, len(hashes), 8)

				err = idx1.Save()
				h.AssertNil(t, err)

				idx2, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii2, ok := idx2.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii2.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), 8)

				// linux/amd64
				imgRefStr := "busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34"
				digest, err := name.NewDigest(imgRefStr, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := ii2.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")
			})
			it("should save all added images with annotations", func() {
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx1, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox:1.36-musl", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx1.Add(
					ref,
					imgutil.WithAll(true),
					imgutil.WithAnnotations(map[string]string{
						"some-key": "some-value",
					}),
				)
				h.AssertNil(t, err)

				ii1, ok := idx1.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				keys := make([]v1.Hash, 0, len(ii1.Images))
				for h2 := range ii1.Images {
					keys = append(keys, h2)
				}
				h.AssertEq(t, len(keys), 8)

				err = idx1.Save()
				h.AssertNil(t, err)

				idx2, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii2, ok := idx2.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii2.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), len(keys))

				// linux/amd64
				var imgRefStr1 = "busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34"
				h.AssertNotEq(t, imgRefStr1, "")
				digest1, err := name.NewDigest(imgRefStr1, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				// linux/arm64
				var imgRefStr2 = "busybox@sha256:648143a312f16e5b5a6f64dfa4024a281fb4a30467500ca8b0091a9984f1c751"
				h.AssertNotEq(t, imgRefStr2, "")
				digest2, err := name.NewDigest(imgRefStr2, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := ii2.OS(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := ii2.Architecture(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				annos, err := ii2.Annotations(digest1)
				h.AssertNil(t, err)

				v, ok := annos["some-key"]
				h.AssertEq(t, ok, true)
				h.AssertEq(t, v, "some-value")

				os, err = ii2.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = ii2.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm64")

				annos, err = ii2.Annotations(digest2)
				h.AssertNil(t, err)

				v, ok = annos["some-key"]
				h.AssertEq(t, ok, true)
				h.AssertEq(t, v, "some-value")
			})
			it("should save platform specific added image", func() {
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox:1.36-musl", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx.Add(ref)
				h.AssertNil(t, err)

				ii, ok := idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				keys := make([]v1.Hash, 0, len(ii.Images))
				for h2 := range ii.Images {
					keys = append(keys, h2)
				}
				h.AssertEq(t, len(keys), 1)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii, ok = idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), len(keys))

				imgRefStr := "busybox@" + mfestSaved.Manifests[0].Digest.String()
				digest, err := name.NewDigest(imgRefStr, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := ii.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, runtime.GOOS)

				arch, err := ii.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, runtime.GOARCH)
			})
			it("should save platform specific added image with annotations", func() {
				if runtime.GOOS == "windows" {
					// TODO we need to prepare a registry image for windows
					t.Skip("busybox is not available for windows")
				}
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox:1.36-musl", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx.Add(ref, imgutil.WithAnnotations(map[string]string{
					"some-key": "some-value",
				}))
				h.AssertNil(t, err)

				ii, ok := idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				keys := make([]v1.Hash, 0, len(ii.Images))
				for h2 := range ii.Images {
					keys = append(keys, h2)
				}
				h.AssertEq(t, len(keys), 1)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii, ok = idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), len(keys))

				imgRefStr := "busybox@" + mfestSaved.Manifests[0].Digest.String()
				digest, err := name.NewDigest(imgRefStr, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := ii.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, runtime.GOOS)

				arch, err := ii.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, runtime.GOARCH)

				annos, err := ii.Annotations(digest)
				h.AssertNil(t, err)

				v, ok := annos["some-key"]
				h.AssertEq(t, ok, true)
				h.AssertEq(t, v, "some-value")
			})
			it("should save target specific added images", func() {
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox:1.36-musl", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx.Add(ref, imgutil.WithOS("linux"), imgutil.WithArchitecture("amd64"))
				h.AssertNil(t, err)

				ii, ok := idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				keys := make([]v1.Hash, 0, len(ii.Images))
				for h2 := range ii.Images {
					keys = append(keys, h2)
				}
				h.AssertEq(t, len(keys), 1)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii, ok = idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), len(keys))

				// linux/amd64
				imgRefStr := "busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34"
				digest, err := name.NewDigest(imgRefStr, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := ii.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := ii.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")
			})
			it("should save target specific added images with Annotations", func() {
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox:1.36-musl", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx.Add(
					ref,
					imgutil.WithOS("linux"),
					imgutil.WithArchitecture("amd64"),
					imgutil.WithAnnotations(map[string]string{
						"some-key": "some-value",
					}),
				)
				h.AssertNil(t, err)

				ii, ok := idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				keys := make([]v1.Hash, 0, len(ii.Images))
				for h2 := range ii.Images {
					keys = append(keys, h2)
				}
				h.AssertEq(t, len(keys), 1)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii, ok = idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), len(keys))

				// linux/amd64
				var imgRefStr1 = "busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34"
				digest, err := name.NewDigest(imgRefStr1, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := ii.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := ii.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				annos, err := ii.Annotations(digest)
				h.AssertNil(t, err)

				v, ok := annos["some-key"]
				h.AssertEq(t, ok, true)
				h.AssertEq(t, v, "some-value")
			})
			it("should save single added image", func() {
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx.Add(ref)
				h.AssertNil(t, err)

				ii, ok := idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				keys := make([]v1.Hash, 0, len(ii.Images))
				for h2 := range ii.Images {
					keys = append(keys, h2)
				}
				h.AssertEq(t, len(keys), 1)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii, ok = idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), 1)

				// linux/amd64
				imgRefStr := "busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34"
				digest, err := name.NewDigest(imgRefStr, name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := ii.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := ii.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")
			})
			it("should save single added image with annotations", func() {
				_, err := index.NewIndex(
					"pack/imgutil",
					index.WithXDGRuntimePath(xdgPath),
					index.WithFormat(types.OCIImageIndex),
				)
				h.AssertNil(t, err)

				idx, err := layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ref, err := name.ParseReference("busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34", name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				err = idx.Add(ref, imgutil.WithAnnotations(map[string]string{
					"some-key": "some-value",
				}))
				h.AssertNil(t, err)

				ii, ok := idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				keys := make([]v1.Hash, 0, len(ii.Images))
				for h2 := range ii.Images {
					keys = append(keys, h2)
				}
				h.AssertEq(t, len(keys), 1)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex("pack/imgutil", index.WithXDGRuntimePath(xdgPath))
				h.AssertNil(t, err)

				ii, ok = idx.(*imgutil.ManifestHandler)
				h.AssertEq(t, ok, true)

				mfestSaved, err := ii.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfestSaved, nil)
				h.AssertEq(t, len(mfestSaved.Manifests), 1)

				digest, ok := ref.(name.Digest)
				h.AssertEq(t, ok, true)

				os, err := ii.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := ii.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				annos, err := ii.Annotations(digest)
				h.AssertNil(t, err)
				v, ok := annos["some-key"]
				h.AssertEq(t, ok, true)
				h.AssertEq(t, v, "some-value")
			})
			it("should save the annotated images", func() {
				idx, err := remote.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.SetOS(digest1, "some-os")
				h.AssertNil(t, err)

				err = idx.SetArchitecture(digest1, "some-arch")
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err := idx.OS(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")

				arch, err := idx.Architecture(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "some-arch")

				variant, err := idx.Variant(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "v6")

				osVersion, err := idx.OSVersion(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err := idx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err := idx.Annotations(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, annotations, map[string]string(nil))

				os, err = idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = idx.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err = idx.Variant(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, variant, "")

				osVersion, err = idx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = idx.Annotations(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, annotations, map[string]string(nil))
			})
			it("should not save annotations for docker image/index", func() {
				idx, err := remote.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.SetAnnotations(digest1, map[string]string{
					"some-key": "some-value",
				})
				h.AssertNil(t, err)

				err = idx.(*imgutil.ManifestHandler).Save()
				h.AssertNil(t, err)

				idx, err = local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err := idx.OS(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := idx.Architecture(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm")

				variant, err := idx.Variant(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "v6")

				osVersion, err := idx.OSVersion(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err := idx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err := idx.Annotations(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest1.Identifier()).Error())
				h.AssertEq(t, annotations, map[string]string(nil))

				os, err = idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = idx.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err = idx.Variant(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, variant, "")

				osVersion, err = idx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = idx.Annotations(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined(types.DockerManifestList, digest2.Identifier()).Error())
				h.AssertEq(t, annotations, map[string]string(nil))
			})
			it("should save the annotated annotations fields", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest1, err := name.NewDigest(
					"busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest2, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.SetAnnotations(digest1, map[string]string{
					"some-key": "some-value",
				})
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err := idx.OS(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := idx.Architecture(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err := idx.Variant(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, variant, "")

				osVersion, err := idx.OSVersion(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err := idx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err := idx.Annotations(digest1)
				h.AssertNil(t, err)
				v, ok := annotations["some-key"]
				h.AssertEq(t, ok, true)
				h.AssertEq(t, v, "some-value")

				os, err = idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = idx.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm")

				variant, err = idx.Variant(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "v6")

				osVersion, err = idx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = idx.Annotations(digest2)
				h.AssertNil(t, err)
				h.AssertNotEq(t, annotations, map[string]string{})
			})
			it("should save the annotated urls", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.SetURLs(digest1, []string{
					"some-urls",
				})
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err := idx.OS(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := idx.Architecture(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm")

				variant, err := idx.Variant(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "v6")

				osVersion, err := idx.OSVersion(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err := idx.URLs(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, urls, []string{
					"some-urls",
				})

				annotations, err := idx.Annotations(digest1)
				h.AssertNil(t, err)
				h.AssertNotEq(t, annotations, map[string]string(nil))

				os, err = idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = idx.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err = idx.Variant(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, variant, "")

				osVersion, err = idx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = idx.Annotations(digest2)
				h.AssertNil(t, err)
				h.AssertNotEq(t, annotations, map[string]string(nil))
			})
			it("should save annotated osFeatures", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.SetOSFeatures(digest1, []string{
					"some-osFeatures",
				})
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				layoutIdx, err := layout.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				os, err := layoutIdx.OS(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := layoutIdx.Architecture(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "arm")

				variant, err := layoutIdx.Variant(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "v6")

				osVersion, err := layoutIdx.OSVersion(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err := layoutIdx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest1.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := layoutIdx.OSFeatures(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{
					"some-osFeatures",
				})

				urls, err := layoutIdx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err := layoutIdx.Annotations(digest1)
				h.AssertNil(t, err)
				h.AssertNotEq(t, annotations, map[string]string(nil))

				os, err = layoutIdx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = layoutIdx.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err = layoutIdx.Variant(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, variant, "")

				osVersion, err = layoutIdx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, osVersion, "")

				features, err = layoutIdx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = layoutIdx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined(types.OCIImageIndex, digest2.Identifier()).Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = layoutIdx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest2.Identifier()).Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = layoutIdx.Annotations(digest2)
				h.AssertNil(t, err)
				h.AssertNotEq(t, annotations, map[string]string(nil))
			})
			it("should remove the images/indexes from save's output", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.Remove(digest1)
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				_, err = idx.OS(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())

				os, err := idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")
			})
			it("should set the Annotate and RemovedManifests to empty slice", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:b9d056b83bb6446fee29e89a7fcf10203c562c1f59586a6e2f39c903597bda34",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.Remove(digest1)
				h.AssertNil(t, err)

				err = idx.SetOS(digest2, "some-os")
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = layout.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath(xdgPath),
				)
				h.AssertNil(t, err)

				_, err = idx.OS(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest1.Identifier()).Error())

				os, err := idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
			it("should return an error", func() {
				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							{}: {
								MediaType: types.DockerConfigJSON,
							},
						},
					},
					Options: imgutil.IndexOptions{
						Reponame: "alpine:latest",
						XdgPath:  xdgPath,
					},
				}

				err := idx.Save()
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(":").Error())
			})
		})
		when("#Push", func() {
			it("should return an error when index is not saved", func() {
				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					Annotate: imgutil.Annotate{
						Instance: map[v1.Hash]v1.Descriptor{
							{}: {
								MediaType: types.DockerConfigJSON,
							},
						},
					},
				}

				err := idx.Push()
				h.AssertEq(t, err.Error(), imgutil.ErrIndexNeedToBeSaved.Error())
			})
			// FIXME: should need to create a mock to push images and indexes
			it("should push index to registry", func() {})
			it("should push with insecure registry when WithInsecure used", func() {})
			it("should delete local image index", func() {})
			it("should annoate index media type before pushing", func() {})
		})
		when("#Inspect", func() {
			it("should return an error", func() {
				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						{},
					},
				}

				mfest, err := idx.Inspect()
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, mfest, "")
			})
			it("should return index manifest", func() {
				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				mfest, err := idx.Inspect()
				h.AssertNil(t, err)
				h.AssertEq(t, mfest, `{
	"schemaVersion": 2,
	"mediaType": "application/vnd.oci.image.index.v1+json",
	"manifests": []
}`)
			})
		})
		when("#Remove", func() {
			it("should return error when invalid digest provided", func() {
				digest := name.Digest{}

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err := idx.Remove(digest)
				h.AssertEq(t, err.Error(), fmt.Sprintf(`cannot parse hash: "%s"`, digest.Identifier()))
			})
			it("should return an error when manifest with given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.ManifestHandler{
					ImageIndex: empty.Index,
				}

				err = idx.Remove(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
			it("should remove the image/index with the given digest", func() {
				_, err := index.NewIndex("some/index", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
				h.AssertNil(t, err)

				idx, err := layout.NewIndex("some/index", index.WithXDGRuntimePath(xdgPath), index.WithFormat(types.OCIImageIndex))
				h.AssertNil(t, err)

				ref, err := name.ParseReference(
					"busybox:1.36-musl",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.Add(ref, imgutil.WithAll(true))
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:0bcc1b827b855c65eaf6e031e894e682b6170160b8a676e1df7527a19d51fb1a",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.Remove(digest)
				h.AssertNil(t, err)

				_, err = idx.OS(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest(digest.Identifier()).Error())
			})
		})
		when("#Delete", func() {
			it("should delete the given index", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithXDGRuntimePath(xdgPath),
					index.WithKeychain(authn.DefaultKeychain),
				)
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				err = idx.Delete()
				h.AssertNil(t, err)
			})
			it("should return an error if the index is already deleted", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithXDGRuntimePath(xdgPath),
					index.WithKeychain(authn.DefaultKeychain),
				)
				h.AssertNil(t, err)

				err = idx.Delete()
				localPath := filepath.Join(xdgPath, imgutil.MakeFileSafeName("busybox:1.36-musl"))
				fmt.Println(err.Error())
				h.AssertEq(t, err.Error(), fmt.Sprintf("stat %s: no such file or directory", localPath))
			})
		})
	})
}
