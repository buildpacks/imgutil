package index

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TextIndex(t *testing.T) {
	spec.Run(t, "IndexTest", testIndex, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testIndex(t *testing.T, when spec.G, it spec.S) {
	when("#NewIndex", func() {
		it("should return new oci Index", func() {

		})
		it("should return new docker Index", func() {

		})
		it("should return an error", func() {

		})
		when("#NewIndex options", func() {
			when("#OS", func() {
				it("should return expected os", func() {})
				it("should return an error", func() {})
			})
			when("#Architecture", func() {
				it("should return expected architecture", func() {})
				it("should return an error", func() {})
			})
			when("#Variant", func() {
				it("should return expected variant", func() {})
				it("should return an error", func() {})
			})
			when("#OSVersion", func() {
				it("should return expected os version", func() {})
				it("should return an error", func() {})
			})
			when("#Features", func() {
				it("should return expected features for image", func() {})
				it("should return expected features for index", func() {})
				it("should return an error", func() {})
			})
			when("#OSFeatures", func() {
				it("should return expected os features for image", func() {})
				it("should return expected os features for index", func() {})
				it("should return an error", func() {})
			})
			when("#Annotations", func() {
				it("should return expected annotations for oci index", func() {})
				it("should return expected annotations for oci image", func() {})
				it("should not return annotations for docker index", func() {})
				it("should not return annotations for docker image", func() {})
				it("should return an error", func() {})
			})
			when("#URLs", func() {
				it("should return expected urls for index", func() {})
				it("should return expected urls for image", func() {})
				it("should return an error", func() {})
			})
			when("#SetOS", func() {
				it("should annotate the image os", func() {})
				it("should return an error", func() {})
			})
			when("#SetArchitecture", func() {
				it("should annotate the image architecture", func() {})
				it("should return an error", func() {})
			})
			when("#SetVariant", func() {
				it("should annotate the image variant", func() {})
				it("should return an error", func() {})
			})
			when("#SetOSVersion", func() {
				it("should annotate the image os version", func() {})
				it("should return an error", func() {})
			})
			when("#SetFeatures", func() {
				it("should annotate the image features", func() {})
				it("should annotate the index features", func() {})
				it("should return an error", func() {})
			})
			when("#SetOSFeatures", func() {
				it("should annotate the image os features", func() {})
				it("should annotate the index os features", func() {})
				it("should return an error", func() {})
			})
			when("#SetAnnotations", func() {
				it("should annotate the image annotations", func() {})
				it("should annotate the index annotations", func() {})
				it("should return an error", func() {})
			})
			when("#SetURLs", func() {
				it("should annotate the image urls", func() {})
				it("should annotate the index urls", func() {})
				it("should return an error", func() {})
			})
			when("#Add", func() {
				it("should add an image", func() {})
				it("should add all images in index", func() {})
				it("should add platform specific image", func() {})
				it("should add target specific image", func() {})
				it("should return an error", func() {})
			})
			when("#Save", func() {
				it("should save image with expected annotated os", func() {})
				it("should save image with expected annotated architecture", func() {})
				it("should save image with expected annotated variant", func() {})
				it("should save image with expected annotated os version", func() {})
				it("should save image with expected annotated features", func() {})
				it("should save image with expected annotated os features", func() {})
				it("should save image with expected annotated annotations for oci", func() {})
				it("should save image without annotations for docker", func() {})
				it("should save image with expected annotated urls", func() {})
				it("should return an error", func() {})
			})
			when("#Push", func() {
				it("should push index to registry", func() {})
				it("should return an error", func() {})
			})
			when("#Inspect", func() {
				it("should return an error", func() {})
				it("should print index raw manifest", func() {})
			})
			when("#Delete", func() {
				it("should delete index from local storage", func() {})
				it("should return an error", func() {})
			})
		})
	})
}
