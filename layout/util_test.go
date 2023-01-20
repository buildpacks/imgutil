package layout_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/imgutil/layout"
	h "github.com/buildpacks/imgutil/testhelpers"
)

const (
	defaultDockerRegistry = name.DefaultRegistry
	defaultDockerRepo     = "library"
)

func Test(t *testing.T) {
	spec.Run(t, "Utilities", test, spec.Sequential(), spec.Report(report.Terminal{}))
}

type testCase struct {
	description  string
	focus        bool
	imageRef     string
	expectedPath string
}

func test(t *testing.T, when spec.G, it spec.S) {
	when("All", func() {
		for _, tc := range []testCase{
			{
				description:  "no registry, repo, tag or digest are provided",
				imageRef:     "my-full-stack-run",
				expectedPath: filepath.Join(defaultDockerRegistry, defaultDockerRepo, "my-full-stack-run", "latest"),
			},
			{
				description:  "tag is provided but no registry or repo",
				imageRef:     tag("my-full-stack-run", "bionic"),
				expectedPath: filepath.Join(defaultDockerRegistry, defaultDockerRepo, "my-full-stack-run", "bionic"),
			},
			{
				description:  "digest is provided but no registry or repo",
				imageRef:     sha256("my-full-stack-run", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
				expectedPath: filepath.Join(defaultDockerRegistry, defaultDockerRepo, "my-full-stack-run", "sha256", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
			},
			{
				description:  "repo is provided but no registry tag or digest",
				imageRef:     "cnb/my-full-stack-run",
				expectedPath: filepath.Join(defaultDockerRegistry, "cnb", "my-full-stack-run", "latest"),
			},
			{
				description:  "repo and tag are provided but no registry",
				imageRef:     tag("cnb/my-full-stack-run", "bionic"),
				expectedPath: filepath.Join(defaultDockerRegistry, "cnb", "my-full-stack-run", "bionic"),
			},
			{
				description:  "repo and digest are provided but no registry",
				imageRef:     sha256("cnb/my-full-stack-run", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
				expectedPath: filepath.Join(defaultDockerRegistry, "cnb", "my-full-stack-run", "sha256", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
			},
			{
				description:  "registry is provided but no repo tag or digest",
				imageRef:     "my-registry.com/my-full-stack-run",
				expectedPath: filepath.Join("my-registry.com", "my-full-stack-run", "latest"),
			},
			{
				description:  "registry and tag are provided but no repo",
				imageRef:     tag("my-registry.com/my-full-stack-run", "bionic"),
				expectedPath: filepath.Join("my-registry.com", "my-full-stack-run", "bionic"),
			},
			{
				description:  "registry and digest are provided but repo",
				imageRef:     sha256("my-registry.com/my-full-stack-run", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
				expectedPath: filepath.Join("my-registry.com", "my-full-stack-run", "sha256", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
			},
			{
				description:  "registry and repo are provided but no tag or digest",
				imageRef:     "my-registry.com/cnb/my-full-stack-run",
				expectedPath: filepath.Join("my-registry.com", "cnb", "my-full-stack-run", "latest"),
			},
			{
				description:  "registry repo and tag are provided",
				imageRef:     tag("my-registry.com/cnb/my-full-stack-run", "bionic"),
				expectedPath: filepath.Join("my-registry.com", "cnb", "my-full-stack-run", "bionic"),
			},
			{
				description:  "registry repo and digest are provided",
				imageRef:     sha256("my-registry.com/cnb/my-full-stack-run", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
				expectedPath: filepath.Join("my-registry.com", "cnb", "my-full-stack-run", "sha256", "f75f3d1a317fc82c793d567de94fc8df2bece37acd5f2bd364a0d91a0d1f3dab"),
			},
		} {
			tc := tc
			w := when
			if tc.focus {
				w = when.Focus
			}
			w(tc.description, func() {
				it("parse image reference to local path", func() {
					path, err := layout.ParseRefToPath(tc.imageRef)
					h.AssertNil(t, err)
					h.AssertEq(t, path, tc.expectedPath)
				})
			})
		}
	})
}

func tag(image, tag string) string {
	return fmt.Sprintf("%s:%s", image, tag)
}

func sha256(image, digest string) string {
	return fmt.Sprintf("%s@sha256:%s", image, digest)
}
