package imgutil_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestIndex(t *testing.T) {
	spec.Run(t, "Index", testIndex, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testIndex(t *testing.T, when spec.G, it spec.S) {
	when("getters", func() {
		when("#OS", func() {
			it("should return os", func() {})
			it("should return empty string", func() {})
			it("should return error", func() {})
		})
		when("#Architecture", func() {
			it("should return architecture", func() {})
			it("should return empty string", func() {})
			it("should return error", func() {})
		})
		when("#Variant", func() {
			it("should return variant", func() {})
			it("should return empty string", func() {})
			it("should return error", func() {})
		})
		when("#OSVersion", func() {
			it("should return os version", func() {})
			it("should return empty string", func() {})
			it("should return error", func() {})
		})
		when("#Features", func() {
			it("should return features", func() {})
			it("should return empty slice", func() {})
			it("should return error", func() {})
		})
		when("#OSFeatures", func() {
			it("should return os features", func() {})
			it("should return empty slice", func() {})
			it("should return error", func() {})
		})
		when("#Annotations", func() {
			it("should return annotations", func() {})
			it("should not return annotaions when format is not oci", func() {})
			it("should return empty map", func() {})
			it("should return error", func() {})
		})
		when("#URLs", func() {
			it("should return urls", func() {})
			it("should return empty slice", func() {})
			it("should return error", func() {})
		})
	})

	when("setter", func() {
		when("#SetOS", func() {
			it("should return an error", func() {})
			it("should annotate os", func() {})
		})
		when("#SetArchitecture", func() {
			it("should return an error", func() {})
			it("should annotate architecture", func() {})
		})
		when("#SetVariant", func() {
			it("should return an error", func() {})
			it("should annotate variant", func() {})
		})
		when("#SetOSVersion", func() {
			it("should return an error", func() {})
			it("should annotate os-version", func() {})
		})
		when("#SetFeatures", func() {
			it("should return an error", func() {})
			it("should annotate features", func() {})
		})
		when("#SetOSFeatures", func() {
			it("should return an error", func() {})
			it("should annotate os-features", func() {})
		})
		when("#SetAnnotations", func() {
			it("should return an error", func() {})
			it("should annotate annotaions", func() {})
		})
		when("#SetURLs", func() {
			it("should return an error", func() {})
			it("should annotate urls", func() {})
		})
	})
	when("misc", func() {
		when("#Add", func() {
			it("should add given image", func() {})
			when("Index requested to add", func() {
				it("should add platform specific image", func() {})
				it("should add all images in index", func() {})
				it("should add image with given OS", func() {})
				it("should add image with the given Arch", func() {})
				it("should return an error", func() {})
			})
			it("should return an error", func() {})
		})
		when("#Save", func() {
			it("should save the image", func() {})
			it("should return an error", func() {})
			when("modify IndexType", func() {
				it("should not have annotaioins for docker manifest list", func() {})
				it("should save index with correct format", func() {})
			})
		})
		when("#Push", func() {
			it("should return an error", func() {})
			it("should push index to secure registry", func() {})
			it("should not push index to insecure registry", func() {})
			it("should push index to insecure registry", func() {})
			it("should change format and push index", func() {})
			it("should delete index from local storage", func() {})
		})
		when("#Inspect", func() {
			it("should return an error", func() {})
			it("should print index content", func() {})
		})
		when("#Remove", func() {
			it("should remove given image", func() {})
			it("should remove all images from given index", func() {})
			it("should return an error", func() {})
		})
		when("#Delete", func() {
			it("should delete given index", func() {})
			it("should return an error", func() {})
		})
	})
	when("#NewIndex", func() {
		it("should load index", func() {})
		it("should return an error", func() {})
	})
}
