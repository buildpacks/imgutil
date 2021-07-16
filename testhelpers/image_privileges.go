package testhelpers

import (
	"regexp"
	"strings"
)

var urlRegex = regexp.MustCompile(`v2\/(.*)\/(?:blobs|manifests|tags)`)

type ImagePrivileges struct {
	readable bool
	writable bool
}

// Creates a new ImagePrivileges, use "readable" or "writable" to set the properties accordingly.
// For examples:
// NewImagePrivileges("") returns ImagePrivileges{readable: false, writable: false}
// NewImagePrivileges("foo-readable") returns ImagePrivileges{readable: true, writable: false}
// NewImagePrivileges("foo-writable") returns ImagePrivileges{readable: false, writable: true}
// NewImagePrivileges("foo-writable-readable") returns ImagePrivileges{readable: true, writable: true}
func NewImagePrivileges(imageName string) (priv ImagePrivileges) {
	priv.readable = strings.Contains(imageName, "readable")
	priv.writable = strings.Contains(imageName, "writable")
	return
}

// Based of the registry API specification https://docs.docker.com/registry/spec/api/
// This method returns the image name for path value that matches requests to blobs, manifests or tags
// For examples:
// extractImageName("v2/foo.bar/blobs/") returns "foo.bar"
// extractImageName("v2/foo/bar/manifests/") returns "foo/bar"
func extractImageName(path string) string {
	var name string
	if strings.Contains(path, "blobs") ||
		strings.Contains(path, "manifests") ||
		strings.Contains(path, "tags") {
		matches := urlRegex.FindStringSubmatch(path)
		name = matches[1]
	}
	return name
}
