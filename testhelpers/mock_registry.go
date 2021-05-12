package testhelpers

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/google/go-containerregistry/pkg/v1/random"
)

type MockServer struct {
	repo        string
	statusCode  int
	failedCount int
	actualCount int
	server      *httptest.Server
}

func NewMockServer(repo string, statusCode, failedCount int) *MockServer {
	return &MockServer{
		repo:        repo,
		statusCode:  statusCode,
		failedCount: failedCount,
		actualCount: 0,
	}
}

func (m *MockServer) Init() *httptest.Server {
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", m.repo)
	img, _ := random.Image(1024, 1)
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.URL.Path == manifestPath {
			m.actualCount++
			if m.actualCount <= m.failedCount {
				w.WriteHeader(m.statusCode)
			} else {
				mm, _ := img.RawManifest()
				_, err = w.Write(mm)
			}
		}
		if err != nil {
			fmt.Printf("There was an error in the mock registry %s\n", err)
		}
	}))
	return m.server
}

func (m *MockServer) ActualCount() int {
	return m.actualCount
}

func (m *MockServer) Server() *httptest.Server {
	return m.server
}
