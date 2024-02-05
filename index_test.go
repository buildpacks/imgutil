package imgutil_test

import (
	"fmt"
	"os"
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
	"github.com/buildpacks/imgutil/docker"
	"github.com/buildpacks/imgutil/index"
	"github.com/buildpacks/imgutil/layout"
	"github.com/buildpacks/imgutil/local"
	"github.com/buildpacks/imgutil/remote"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestIndex(t *testing.T) {
	spec.Run(t, "Index", testIndex, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testIndex(t *testing.T, when spec.G, it spec.S) {
	when("#ImageIndex", func() {
		it.After(func() {
			err := os.RemoveAll("xdgPath")
			h.AssertNil(t, err)
		})
		when("#OS", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.Index{}
				_, err := idx.OS(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #OS requested", func() {
				digest, err := name.NewDigest("busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				os, err := idx.OS(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, os, "")
			})
			it("should return latest OS when os of the given digest annotated", func() {
				digest, err := name.NewDigest("busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
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
				digest, err := name.NewDigest("busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				os, err := idx.OS(digest)
				h.AssertEq(t, err.Error(), "empty index")
				h.AssertEq(t, os, "")
			})
			it("should return expected os when os is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
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
				idx := imgutil.Index{}
				err := idx.SetOS(digest, "some-os")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetOS requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetOS(digest, "some-os")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
			it("should SetOS for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:latest",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.ImageIndex.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash := mfest.Manifests[0].Digest
				digest, err := name.NewDigest("alpine@" + hash.String())
				h.AssertNil(t, err)

				err = imgIdx.SetOS(digest, "some-os")
				h.AssertNil(t, err)

				os, err := imgIdx.OS(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.SetOS(digest, "some-os")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("#Architecture", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.Index{}
				_, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #Architecture requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				os, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, os, "")
			})
			it("should return latest Architecture when arch of the given digest annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
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
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				arch, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), "empty index")
				h.AssertEq(t, arch, "")
			})
			it("should return expected Architecture when arch is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath("xdgPath"))
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
				idx := imgutil.Index{}
				err := idx.SetArchitecture(digest, "some-arch")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetArchitecture requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetArchitecture(digest, "some-arch")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
			it("should SetArchitecture for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:latest",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.ImageIndex.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash := mfest.Manifests[0].Digest
				digest, err := name.NewDigest("alpine@" + hash.String())
				h.AssertNil(t, err)

				err = imgIdx.SetArchitecture(digest, "some-arch")
				h.AssertNil(t, err)

				os, err := imgIdx.Architecture(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-arch")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.SetArchitecture(digest, "some-arch")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("#Variant", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.Index{}
				_, err := idx.Architecture(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #Variant requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				variant, err := idx.Variant(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, variant, "")
			})
			it("should return latest Variant when variant of the given digest annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
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
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				arch, err := idx.Variant(digest)
				h.AssertEq(t, err.Error(), "empty index")
				h.AssertEq(t, arch, "")
			})
			it("should return expected Variant when arch is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath("xdgPath"))
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
				idx := imgutil.Index{}
				err := idx.SetVariant(digest, "some-variant")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetVariant requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetVariant(digest, "some-variant")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
			it("should SetVariant for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:latest",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.ImageIndex.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash := mfest.Manifests[0].Digest
				digest, err := name.NewDigest("alpine@" + hash.String())
				h.AssertNil(t, err)

				err = imgIdx.SetVariant(digest, "some-variant")
				h.AssertNil(t, err)

				os, err := imgIdx.Variant(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-variant")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.SetVariant(digest, "some-variant")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("#OSVersion", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.Index{}
				_, err := idx.OSVersion(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #OSVersion requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				osVersion, err := idx.OSVersion(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, osVersion, "")
			})
			it("should return latest OSVersion when osVersion of the given digest annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
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
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				osVersion, err := idx.OSVersion(digest)
				h.AssertEq(t, err.Error(), "empty index")
				h.AssertEq(t, osVersion, "")
			})
			it("should return expected OSVersion when arch is not annotated before", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath("xdgPath"))
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
				idx := imgutil.Index{}
				err := idx.SetOSVersion(digest, "some-osVersion")
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error if a removed image/index's #SetOSVersion requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetOSVersion(digest, "some-osVersion")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
			it("should SetOSVersion for the given digest when image/index exists", func() {
				idx, err := remote.NewIndex(
					"busybox:latest",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.ImageIndex.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash := mfest.Manifests[0].Digest
				digest, err := name.NewDigest("alpine@" + hash.String())
				h.AssertNil(t, err)

				err = imgIdx.SetOSVersion(digest, "some-osVersion")
				h.AssertNil(t, err)

				os, err := imgIdx.OSVersion(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-osVersion")
			})
			it("it should return an error when image/index with the given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.SetOSVersion(digest, "some-osVersion")
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("#Features", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.Index{}
				_, err := idx.Features(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #Features is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				features, err := idx.Features(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, features, []string(nil))
			})
			it("should return annotated Features when Features of the image/index is annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
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
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				features, err := idx.Features(digest)
				h.AssertEq(t, err.Error(), "empty index")
				h.AssertEq(t, features, []string(nil))
			})
			it("should return expected Features of the given image/index when image/index is not annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath("xdgPath"))
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
				idx := imgutil.Index{}
				err := idx.SetFeatures(digest, []string{"some-features"})
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #SetFeatures is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetFeatures(digest, []string{"some-features"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
			it("should SetFeatures when the given digest is image/index", func() {
				idx, err := remote.NewIndex(
					"busybox:latest",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.ImageIndex.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash := mfest.Manifests[0].Digest
				digest, err := name.NewDigest("alpine@" + hash.String())
				h.AssertNil(t, err)

				err = imgIdx.SetFeatures(digest, []string{"some-features"})
				h.AssertNil(t, err)

				features, err := imgIdx.Features(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, features, []string{"some-features"})
			})
			it("should return an error when no image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.SetFeatures(digest, []string{"some-features"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("#OSFeatures", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.Index{}
				_, err := idx.OSFeatures(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #OSFeatures is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				osFeatures, err := idx.OSFeatures(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, osFeatures, []string(nil))
			})
			it("should return annotated OSFeatures when OSFeatures of the image/index is annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
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
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				osFeatures, err := idx.OSFeatures(digest)
				h.AssertEq(t, err.Error(), "empty index")
				h.AssertEq(t, osFeatures, []string(nil))
			})
			it("should return expected OSFeatures of the given image when image/index is not annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath("xdgPath"))
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
				idx := imgutil.Index{}
				err := idx.SetFeatures(digest, []string{"some-osFeatures"})
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #SetOSFeatures is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
			it("should SetOSFeatures when the given digest is image/index", func() {
				idx, err := remote.NewIndex(
					"busybox:latest",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.ImageIndex.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash := mfest.Manifests[0].Digest
				digest, err := name.NewDigest("alpine@" + hash.String())
				h.AssertNil(t, err)

				err = imgIdx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertNil(t, err)

				osFeatures, err := imgIdx.OSFeatures(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{"some-osFeatures"})
			})
			it("should return an error when no image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.SetOSFeatures(digest, []string{"some-osFeatures"})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("docker manifest list", func() {
			when("#Annotations", func() {
				it("should return an error when invalid digest provided", func() {
					digest := name.Digest{}
					idx := imgutil.Index{}
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

					idx := imgutil.Index{
						ImageIndex: docker.DockerIndex,
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
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
						index.WithXDGRuntimePath("xdgPath"),
					)
					h.AssertNil(t, err)

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return the Annotations if the image/index with the given digest exists", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.Index{
						ImageIndex: docker.DockerIndex,
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), "empty index")
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return expected Annotations of the given image/index when image/index is not annotated", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx, err := remote.NewIndex("alpine:3.19.0", index.WithXDGRuntimePath("xdgPath"))
					h.AssertNil(t, err)
					h.AssertNotEq(t, idx, v1.ImageIndex(nil))

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertNil(t, err)

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
			})
			when("#SetAnnotations", func() {
				it("should return an error when invalid digest provided", func() {
					digest := name.Digest{}
					idx := imgutil.Index{}
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

					idx := imgutil.Index{
						ImageIndex: docker.DockerIndex,
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				})
				it("should SetAnnotations when an image/index with the given digest exists", func() {
					idx, err := remote.NewIndex(
						"alpine:latest",
						index.WithInsecure(true),
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath("xdgPath"),
					)
					h.AssertNil(t, err)

					imgIdx, ok := idx.(*imgutil.Index)
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
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return an error if the manifest with the given digest is neither image nor index", func() {
					digest, err := name.NewDigest(
						"alpine@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.Index{
						ImageIndex: docker.DockerIndex,
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				})
			})
		})
		when("oci image index", func() {
			when("#Annotations", func() {
				it("should return an error when invalid digest provided", func() {
					digest := name.Digest{}
					idx := imgutil.Index{}
					_, err := idx.OSFeatures(digest)
					h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
				})
				it("should return an error when a removed manifest's #Annotations is requested", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					hash, err := v1.NewHash(digest.Identifier())
					h.AssertNil(t, err)

					idx := imgutil.Index{
						ImageIndex: empty.Index,
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return annotated Annotations when Annotations of the image/index is annotated", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx, err := remote.NewIndex(
						"busybox:1.36-musl",
						index.WithInsecure(true),
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath("xdgPath"),
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
						"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.Index{
						ImageIndex: empty.Index,
					}

					annotations, err := idx.Annotations(digest)
					h.AssertEq(t, err.Error(), "empty index")
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should return expected Annotations of the given image when image/index is not annotated", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath("xdgPath"))
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
					idx := imgutil.Index{}
					err := idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
				})
				it("should return an error if the image/index is removed", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					hash, err := v1.NewHash(digest.Identifier())
					h.AssertNil(t, err)

					idx := imgutil.Index{
						ImageIndex: empty.Index,
						RemovedManifests: []v1.Hash{
							hash,
						},
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				})
				it("should SetAnnotations when an image/index with the given digest exists", func() {
					idx, err := remote.NewIndex(
						"busybox:latest",
						index.WithInsecure(true),
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath("xdgPath"),
					)
					h.AssertNil(t, err)

					imgIdx, ok := idx.(*imgutil.Index)
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
					h.AssertNil(t, err)
					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should return an error if the manifest with the given digest is neither image nor index", func() {
					digest, err := name.NewDigest(
						"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					idx := imgutil.Index{
						ImageIndex: empty.Index,
					}

					err = idx.SetAnnotations(digest, map[string]string{
						"some-key": "some-value",
					})
					h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				})
			})
		})
		when("#URLs", func() {
			it("should return an error when invalid digest provided", func() {
				digest := name.Digest{}
				idx := imgutil.Index{}
				_, err := idx.URLs(digest)
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #URLs is requested", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				urls, err := idx.URLs(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, urls, []string(nil))
			})
			it("should return annotated URLs when URLs of the image/index is annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
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
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				urls, err := idx.URLs(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
				h.AssertEq(t, urls, []string(nil))
			})
			it("should return expected URLs of the given image when image/index is not annotated", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx, err := remote.NewIndex("busybox:1.36-musl", index.WithXDGRuntimePath("xdgPath"))
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
				idx := imgutil.Index{}
				err := idx.SetURLs(digest, []string{"some-urls"})
				h.AssertEq(t, err.Error(), fmt.Errorf(`cannot parse hash: "%s"`, digest.Identifier()).Error())
			})
			it("should return an error when a removed manifest's #SetURLs is requested", func() {
				digest, err := name.NewDigest("busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b", name.WeakValidation, name.Insecure)
				h.AssertNil(t, err)

				hash, err := v1.NewHash(digest.Identifier())
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						hash,
					},
				}

				err = idx.SetURLs(digest, []string{
					"some-urls",
				})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
			it("should SetOSFeatures when the given digest is image/index", func() {
				idx, err := remote.NewIndex(
					"busybox:latest",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.ImageIndex.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash := mfest.Manifests[0].Digest
				digest, err := name.NewDigest("alpine@" + hash.String())
				h.AssertNil(t, err)

				err = imgIdx.SetURLs(digest, []string{
					"some-urls",
				})
				h.AssertNil(t, err)

				urls, err := imgIdx.URLs(digest)
				h.AssertNil(t, err)
				h.AssertEq(t, urls, []string{
					"some-urls",
				})
			})
			it("should return an error when no image/index with the given digest exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.SetURLs(digest, []string{
					"some-urls",
				})
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("#Add", func() {
			it("should return an error when the image/index with the given reference doesn't exists", func() {
				_, err := remote.NewIndex(
					"unknown/index",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertEq(t, err.Error(), "GET https://index.docker.io/v2/unknown/index/manifests/latest: UNAUTHORIZED: authentication required; [map[Action:pull Class: Name:unknown/index Type:repository]]")
			})
			when("platform specific", func() {
				it("should add platform specific image", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:latest",
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

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should add annotations when WithAnnotations used for oci", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
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

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("busybox@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertNil(t, err)

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should not add annotations when WithAnnotations used for docker", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:latest",
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

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
			})
			when("target specific", func() {
				it("should add target specific image", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine:latest",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(ref)
					h.AssertNil(t, err)

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, runtime.GOOS)

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, runtime.GOARCH)

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should add annotations when WithAnnotations used for oci", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
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

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("busybox@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, runtime.GOOS)

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, runtime.GOARCH)

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertNil(t, err)

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should not add annotations when WithAnnotations used for docker", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
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

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, runtime.GOOS)

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, runtime.GOARCH)

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
			})
			when("image specific", func() {
				it("should not change the digest of the image when added", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"alpine@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					err = idx.Add(ref)
					h.AssertNil(t, err)

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
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
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should annotate the annotations when Annotations provided for oci", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"busybox@sha256:fed6b26ea319254ef0d6bae87482b5ab58b85250a7cc46d14c533e1f5c2556db",
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

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("busybox@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "arm64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertNil(t, err)

					v, ok := annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should not annotate the annotations when Annotations provided for docker", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
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

					index := idx.(*imgutil.Index)
					mfest, err := index.IndexManifest()
					h.AssertNil(t, err)
					h.AssertNotEq(t, mfest, nil)
					h.AssertEq(t, len(mfest.Manifests), 1)

					hash := mfest.Manifests[0].Digest
					digest, err := name.NewDigest("alpine@"+hash.String(), name.WeakValidation, name.Insecure)
					h.AssertNil(t, err)

					os, err := index.OS(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := index.Architecture(digest)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := index.Variant(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := index.OSVersion(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := index.Features(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := index.OSFeatures(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := index.URLs(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := index.Annotations(digest)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())

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
						index.WithXDGRuntimePath("xdgPath"),
						index.WithFormat(types.DockerManifestList),
					)
					h.AssertNil(t, err)

					idx, err := local.NewIndex(
						"some/image:tag",
						index.WithKeychain(authn.DefaultKeychain),
						index.WithXDGRuntimePath("xdgPath"),
					)
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

					err = idx.Add(ref, imgutil.WithAll(true))
					h.AssertNil(t, err)

					os, err := idx.OS(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, os, "linux")

					arch, err := idx.Architecture(digest1)
					h.AssertNil(t, err)
					h.AssertEq(t, arch, "amd64")

					variant, err := idx.Variant(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := idx.OSVersion(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := idx.Features(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := idx.OSFeatures(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := idx.URLs(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := idx.Annotations(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
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
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err = idx.Features(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err = idx.OSFeatures(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err = idx.URLs(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err = idx.Annotations(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
					h.AssertEq(t, annotations, map[string]string(nil))
				})
				it("should not ignore WithAnnotations for oci", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
					h.AssertNil(t, err)

					ref, err := name.ParseReference(
						"busybox:1.36-musl",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					digest1, err := name.NewDigest(
						"busybox@sha256:d4707523ce6e12afdbe9a3be5ad69027150a834870ca0933baf7516dd1fe0f56",
						name.WeakValidation,
						name.Insecure,
					)
					h.AssertNil(t, err)

					digest2, err := name.NewDigest(
						"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
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
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := idx.OSVersion(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := idx.Features(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := idx.OSFeatures(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := idx.URLs(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
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
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err = idx.Features(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err = idx.OSFeatures(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err = idx.URLs(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err = idx.Annotations(digest2)
					h.AssertNil(t, err)

					v, ok = annotations["some-key"]
					h.AssertEq(t, ok, true)
					h.AssertEq(t, v, "some-value")
				})
				it("should ignore WithAnnotations for docker", func() {
					idx, err := index.NewIndex("some/image:tag", index.WithXDGRuntimePath("xdgPath"))
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
					h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
					h.AssertEq(t, variant, "")

					osVersion, err := idx.OSVersion(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err := idx.Features(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err := idx.OSFeatures(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err := idx.URLs(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err := idx.Annotations(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
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
					h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
					h.AssertEq(t, osVersion, "")

					features, err = idx.Features(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
					h.AssertEq(t, features, []string(nil))

					osFeatures, err = idx.OSFeatures(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
					h.AssertEq(t, osFeatures, []string(nil))

					urls, err = idx.URLs(digest2)
					h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
					h.AssertEq(t, urls, []string(nil))

					annotations, err = idx.Annotations(digest1)
					h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				err = idx.Save()
				h.AssertNil(t, err)

				_, err = local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)
			})
			it("should save the annotated images", func() {
				idx, err := remote.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash1 := mfest.Manifests[len(mfest.Manifests)-1].Digest
				digest1, err = name.NewDigest("alpine@"+hash1.String(), name.Insecure, name.WeakValidation)
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
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err := idx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err := idx.Annotations(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
				h.AssertEq(t, annotations, map[string]string(nil))

				os, err = idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = idx.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err = idx.Variant(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
				h.AssertEq(t, variant, "")

				osVersion, err = idx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = idx.Annotations(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
				h.AssertEq(t, annotations, map[string]string(nil))
			})
			it("should not save annotations for docker image/index", func() {
				idx, err := remote.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
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

				err = idx.Save()
				h.AssertNil(t, err)

				idx, err = local.NewIndex(
					"alpine:3.19.0",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash1 := mfest.Manifests[len(mfest.Manifests)-1].Digest
				digest1, err = name.NewDigest("alpine@"+hash1.String(), name.Insecure, name.WeakValidation)
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
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err := idx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err := idx.Annotations(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
				h.AssertEq(t, annotations, map[string]string(nil))

				os, err = idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err = idx.Architecture(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err = idx.Variant(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
				h.AssertEq(t, variant, "")

				osVersion, err = idx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = idx.Annotations(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrAnnotationsUndefined.Error())
				h.AssertEq(t, annotations, map[string]string(nil))
			})
			it("should save the annotated annotations fields", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				// linux/ard64
				digest1, err := name.NewDigest(
					"busybox@sha256:d4707523ce6e12afdbe9a3be5ad69027150a834870ca0933baf7516dd1fe0f56",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest2, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash1 := mfest.Manifests[len(mfest.Manifests)-1].Digest
				digest1, err = name.NewDigest("alpine@"+hash1.String(), name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				os, err := idx.OS(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")

				arch, err := idx.Architecture(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "amd64")

				variant, err := idx.Variant(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
				h.AssertEq(t, variant, "")

				osVersion, err := idx.OSVersion(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err := idx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
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
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
				h.AssertEq(t, urls, []string(nil))

				annotations, err = idx.Annotations(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, annotations, map[string]string{
					"org.opencontainers.image.revision": "2ef3ae50941f78eb12b4390e6061872eb6cd265e",
					"org.opencontainers.image.source":   "https://github.com/docker-library/busybox.git#2ef3ae50941f78eb12b4390e6061872eb6cd265e:latest/musl",
					"org.opencontainers.image.url":      "https://hub.docker.com/_/busybox",
					"org.opencontainers.image.version":  "1.36.1-musl",
				})
			})
			it("should save the annotated urls", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:d4707523ce6e12afdbe9a3be5ad69027150a834870ca0933baf7516dd1fe0f56",
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash1 := mfest.Manifests[len(mfest.Manifests)-1].Digest
				digest1, err = name.NewDigest("alpine@"+hash1.String(), name.Insecure, name.WeakValidation)
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
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err := idx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := idx.OSFeatures(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
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
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
				h.AssertEq(t, variant, "")

				osVersion, err = idx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err = idx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = idx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = idx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:d4707523ce6e12afdbe9a3be5ad69027150a834870ca0933baf7516dd1fe0f56",
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := layoutIdx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash1 := mfest.Manifests[len(mfest.Manifests)-1].Digest
				digest1, err = name.NewDigest("alpine@"+hash1.String(), name.Insecure, name.WeakValidation)
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
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err := layoutIdx.Features(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err := layoutIdx.OSFeatures(digest1)
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{
					"some-osFeatures",
				})

				urls, err := layoutIdx.URLs(digest1)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
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
				h.AssertEq(t, err.Error(), imgutil.ErrVariantUndefined.Error())
				h.AssertEq(t, variant, "")

				osVersion, err = layoutIdx.OSVersion(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSVersionUndefined.Error())
				h.AssertEq(t, osVersion, "")

				features, err = layoutIdx.Features(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrFeaturesUndefined.Error())
				h.AssertEq(t, features, []string(nil))

				osFeatures, err = layoutIdx.OSFeatures(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrOSFeaturesUndefined.Error())
				h.AssertEq(t, osFeatures, []string(nil))

				urls, err = layoutIdx.URLs(digest2)
				h.AssertEq(t, err.Error(), imgutil.ErrURLsUndefined.Error())
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:d4707523ce6e12afdbe9a3be5ad69027150a834870ca0933baf7516dd1fe0f56",
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				_, err = idx.OS(digest1)
				h.AssertEq(t, err.Error(), "could not find descriptor in index: sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b")

				os, err := idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "linux")
			})
			it("should set the Annotate and RemovedManifests to empty slice", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithKeychain(authn.DefaultKeychain),
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				// linux/arm/v6
				digest1, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				// linux/amd64
				digest2, err := name.NewDigest(
					"busybox@sha256:d4707523ce6e12afdbe9a3be5ad69027150a834870ca0933baf7516dd1fe0f56",
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
					index.WithXDGRuntimePath("xdgPath"),
				)
				h.AssertNil(t, err)

				imgIdx, ok := idx.(*imgutil.Index)
				h.AssertEq(t, ok, true)

				mfest, err := imgIdx.IndexManifest()
				h.AssertNil(t, err)
				h.AssertNotEq(t, mfest, nil)

				hash1 := mfest.Manifests[len(mfest.Manifests)-1].Digest
				digest2, err = name.NewDigest("alpine@"+hash1.String(), name.Insecure, name.WeakValidation)
				h.AssertNil(t, err)

				_, err = idx.OS(digest1)
				h.AssertEq(t, err.Error(), fmt.Sprintf("could not find descriptor in index: %s", digest1.Identifier()))

				os, err := idx.OS(digest2)
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
			it("should return an error", func() {
				idx := imgutil.Index{
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
					},
				}

				err := idx.Save()
				h.AssertEq(t, err.Error(), imgutil.ErrUnknownMediaType.Error())
			})
		})
		when("#Push", func() {
			it("should return an error when index is not saved", func() {
				idx := imgutil.Index{
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
			it("should push index to registry", func() {})
			it("should push with insecure registry when WithInsecure used", func() {})
			it("should delete local image index", func() {})
			it("should annoate index media type before pushing", func() {})
		})
		when("#Inspect", func() {
			it("should return an error", func() {
				idx := imgutil.Index{
					ImageIndex: empty.Index,
					RemovedManifests: []v1.Hash{
						{},
					},
				}

				err := idx.Inspect()
				h.AssertNotEq(t, err, nil)
			})
			it("should return an error with body of index manifest", func() {
				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err := idx.Inspect()
				h.AssertEq(t, err.Error(), `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[]}`)
			})
		})
		when("#Remove", func() {
			it("should return error when invalid digest provided", func() {
				digest := name.Digest{}

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err := idx.Remove(digest)
				h.AssertEq(t, err.Error(), fmt.Sprintf(`cannot parse hash: "%s"`, digest.Identifier()))
			})
			it("should return an error when manifest with given digest doesn't exists", func() {
				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				err = idx.Remove(digest)
				h.AssertEq(t, err.Error(), "empty index")
			})
			it("should remove the image/index with the given digest", func() {
				idx := imgutil.Index{
					ImageIndex: empty.Index,
				}

				ref, err := name.ParseReference(
					"busybox:1.36-musl",
					name.Insecure,
					name.WeakValidation,
				)
				h.AssertNil(t, err)

				err = idx.Add(ref, imgutil.WithAll(true))
				h.AssertNil(t, err)

				digest, err := name.NewDigest(
					"busybox@sha256:b64a6a9cff5d2916ce4e5ab52254faa487ae93d9028c157c10d444aa3b5b7e4b",
					name.WeakValidation,
					name.Insecure,
				)
				h.AssertNil(t, err)

				err = idx.Remove(digest)
				h.AssertNil(t, err)

				_, err = idx.OS(digest)
				h.AssertEq(t, err.Error(), imgutil.ErrNoImageOrIndexFoundWithGivenDigest.Error())
			})
		})
		when("#Delete", func() {
			it("should delete the given index", func() {
				idx, err := remote.NewIndex(
					"busybox:1.36-musl",
					index.WithInsecure(true),
					index.WithXDGRuntimePath("xdgPath"),
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
					index.WithXDGRuntimePath("xdgPath"),
					index.WithKeychain(authn.DefaultKeychain),
				)
				h.AssertNil(t, err)

				err = idx.Delete()
				h.AssertNil(t, err)
			})
		})
	})
	when("Annotate", func() {
		annotate := imgutil.Annotate{
			Instance: map[v1.Hash]v1.Descriptor{},
		}
		it.Before(func() {
			annotate = imgutil.Annotate{
				Instance: map[v1.Hash]v1.Descriptor{},
			}
		})
		when("#OS", func() {
			it.Before(func() {
				annotate.SetOS(v1.Hash{}, "some-os")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetOS(v1.Hash{}, "")
				os, err := annotate.OS(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, os, "")
			})
			it("should return expected os", func() {
				os, err := annotate.OS(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, os, "some-os")
			})
		})
		when("#Architecture", func() {
			it.Before(func() {
				annotate.SetArchitecture(v1.Hash{}, "some-arch")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetArchitecture(v1.Hash{}, "")
				arch, err := annotate.Architecture(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, arch, "")
			})
			it("should return expected os", func() {
				arch, err := annotate.Architecture(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, arch, "some-arch")
			})
		})
		when("#Variant", func() {
			it.Before(func() {
				annotate.SetVariant(v1.Hash{}, "some-variant")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetVariant(v1.Hash{}, "")
				variant, err := annotate.Variant(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, variant, "")
			})
			it("should return expected os", func() {
				variant, err := annotate.Variant(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, variant, "some-variant")
			})
		})
		when("#OSVersion", func() {
			it.Before(func() {
				annotate.SetOSVersion(v1.Hash{}, "some-osVersion")
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetOSVersion(v1.Hash{}, "")
				osVersion, err := annotate.OSVersion(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, osVersion, "")
			})
			it("should return expected os", func() {
				osVersion, err := annotate.OSVersion(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, osVersion, "some-osVersion")
			})
		})
		when("#Features", func() {
			it.Before(func() {
				annotate.SetFeatures(v1.Hash{}, []string{"some-features"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetFeatures(v1.Hash{}, []string(nil))
				features, err := annotate.Features(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, features, []string(nil))
			})
			it("should return expected features", func() {
				os, err := annotate.Features(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, os, []string{"some-features"})
			})
		})
		when("#OSFeatures", func() {
			it.Before(func() {
				annotate.SetOSFeatures(v1.Hash{}, []string{"some-osFeatures"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetOSFeatures(v1.Hash{}, []string(nil))
				osFeatures, err := annotate.OSFeatures(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, osFeatures, []string(nil))
			})
			it("should return expected os", func() {
				osFeatures, err := annotate.OSFeatures(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, osFeatures, []string{"some-osFeatures"})
			})
		})
		when("#Annotations", func() {
			it.Before(func() {
				annotate.SetAnnotations(v1.Hash{}, map[string]string{"some-key": "some-value"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetAnnotations(v1.Hash{}, map[string]string(nil))
				annotations, err := annotate.Annotations(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, annotations, map[string]string(nil))
			})
			it("should return expected os", func() {
				annotations, err := annotate.Annotations(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, annotations, map[string]string{"some-key": "some-value"})
			})
		})
		when("#URLs", func() {
			it.Before(func() {
				annotate.SetURLs(v1.Hash{}, []string{"some-urls"})
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
			})
			it("should return an error", func() {
				annotate.SetURLs(v1.Hash{}, []string(nil))
				urls, err := annotate.URLs(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, urls, []string(nil))
			})
			it("should return expected os", func() {
				os, err := annotate.URLs(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, os, []string{"some-urls"})
			})
		})
		when("#Format", func() {
			it.Before(func() {
				annotate.SetFormat(v1.Hash{}, types.OCIImageIndex)
				desc, ok := annotate.Instance[v1.Hash{}]
				h.AssertEq(t, ok, true)
				h.AssertNotEq(t, desc, nil)
				h.AssertEq(t, desc.MediaType, types.OCIImageIndex)
			})
			it("should return an error", func() {
				annotate.SetFormat(v1.Hash{}, types.MediaType(""))
				format, err := annotate.Format(v1.Hash{})
				h.AssertNotEq(t, err, nil)
				h.AssertEq(t, format, types.MediaType(""))
			})
			it("should return expected os", func() {
				format, err := annotate.Format(v1.Hash{})
				h.AssertNil(t, err)
				h.AssertEq(t, format, types.OCIImageIndex)
			})
		})
	})
}
