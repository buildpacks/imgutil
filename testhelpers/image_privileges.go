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

func NewImagePrivileges(imageName string) (priv ImagePrivileges) {
	priv.readable = strings.Contains(imageName, "readable")
	priv.writable = strings.Contains(imageName, "writable")
	return
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
