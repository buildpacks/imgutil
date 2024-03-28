package imgutil_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil"
	h "github.com/buildpacks/imgutil/testhelpers"
)

func TestIndexOptions(t *testing.T) {
	spec.Run(t, "IndexOptions", testIndexOptions, spec.Sequential(), spec.Report(report.Terminal{}))
}

var (
	indexOptions = imgutil.IndexOptions{
		XdgPath:          "/xdgPath",
		Reponame:         "some/repoName",
		InsecureRegistry: true,
	}
	addOptions  = &imgutil.AddOptions{}
	pushOptions = &imgutil.PushOptions{}
)

func testIndexOptions(t *testing.T, when spec.G, it spec.S) {
	when("#IndexOption", func() {
		it("#XDGRuntimePath should return expected XDGRuntimePath", func() {
			h.AssertEq(t, indexOptions.XDGRuntimePath(), "/xdgPath")
		})
		it("#RepoName should return expected RepoName", func() {
			h.AssertEq(t, indexOptions.RepoName(), "some/repoName")
		})
		it("#Insecure should return expected boolean", func() {
			h.AssertEq(t, indexOptions.Insecure(), true)
		})
		it("#Keychain should return expected Keychain", func() {
			h.AssertEq(t, indexOptions.Keychain(), nil)
		})
	})
	when("#AddOptions", func() {
		it.Before(func() {
			addOptions = &imgutil.AddOptions{}
		})
		it("#WithAll", func() {
			op := imgutil.WithAll(true)
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
		it("#WithOS", func() {
			op := imgutil.WithOS("some-os")
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
		it("#WithArchitecture", func() {
			op := imgutil.WithArchitecture("some-arch")
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
		it("#WithVariant", func() {
			op := imgutil.WithVariant("some-variant")
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
		it("#WithOSVersion", func() {
			op := imgutil.WithOSVersion("some-osVersion")
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
		it("#WithFeatures", func() {
			op := imgutil.WithFeatures([]string{"some-features"})
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
		it("#WithOSFeatures", func() {
			op := imgutil.WithOSFeatures([]string{"some-osFeatures"})
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
		it("#WithAnnotations", func() {
			op := imgutil.WithAnnotations(map[string]string{"some-key": "some-value"})
			op(addOptions)
			h.AssertNotEq(t, addOptions, imgutil.AddOptions{})
		})
	})
	when("#PushOptions", func() {
		it.Before(func() {
			pushOptions = &imgutil.PushOptions{}
		})
		it("#WithInsecure", func() {
			op := imgutil.WithInsecure(true)
			h.AssertNil(t, op(pushOptions))
			h.AssertEq(t, pushOptions.Insecure, true)
		})
		it("#WithPurge", func() {
			op := imgutil.WithPurge(true)
			h.AssertNil(t, op(pushOptions))
			h.AssertEq(t, pushOptions.Purge, true)
		})
		it("#WithFormat", func() {
			format := types.OCIImageIndex
			op := imgutil.WithFormat(format)
			h.AssertNil(t, op(pushOptions))
			h.AssertEq(t, pushOptions.Format, format)
		})
		it("#WithFormat error", func() {
			op := imgutil.WithFormat(types.OCIConfigJSON)
			h.AssertNotEq(t, op(pushOptions), nil)
			h.AssertEq(t, pushOptions.Format, types.MediaType(""))
		})
		it("#WithTags", func() {
			tags := []string{"latest", "0.0.1", "1.0.0"}
			op := imgutil.WithTags(tags...)
			h.AssertNil(t, op(pushOptions))
			h.AssertEq(t, pushOptions.Tags, tags)
		})
	})
}
