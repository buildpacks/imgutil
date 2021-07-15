package testhelpers

import (
	"regexp"
	"strings"
)

var urlRegex = regexp.MustCompile(`v2\/(.*)\/(?:blobs|manifests|tags)`)

type ImagePrivileges struct {
	readable  bool
	writeable bool
}

func NewImagePrivileges(imageName string) *ImagePrivileges {
	var isReadable, isWriteable bool
	if strings.Contains(imageName, "readable") {
		isReadable = true
	}
	if strings.Contains(imageName, "writable") {
		isWriteable = true
	}
	return &ImagePrivileges{readable: isReadable, writeable: isWriteable}
}

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
