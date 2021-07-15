package testhelpers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
)

type DockerRegistry struct {
	Host             string
	Port             string
	Name             string
	server           *httptest.Server
	DockerDirectory  string
	username         string
	password         string
	regHandler       http.Handler
	authnHandler     http.Handler
	customPrivileges map[string]*ImagePrivileges
}

type RegistryOption func(registry *DockerRegistry)

//WithSharedHandler allows two instances to share the same data by re-using the registry handler.
//Use an authenticated registry to write to a read-only unauthenticated registry.
func WithSharedHandler(handler http.Handler) RegistryOption {
	return func(registry *DockerRegistry) {
		registry.regHandler = handler
	}
}

//WithCustomPrivileges allows to execute some customer read/write access validations based on the image name
func WithCustomPrivileges(permissions map[string]*ImagePrivileges) RegistryOption {
	return func(registry *DockerRegistry) {
		registry.customPrivileges = permissions
	}
}

//WithAuth adds credentials to registry. Omitting will make the registry read-only
func WithAuth(dockerConfigDir string) RegistryOption {
	return func(r *DockerRegistry) {
		r.username = RandString(10)
		r.password = RandString(10)
		r.DockerDirectory = dockerConfigDir
	}
}

func NewDockerRegistry(ops ...RegistryOption) *DockerRegistry {
	registry := &DockerRegistry{
		Name: "test-registry-" + RandString(10),
	}

	for _, op := range ops {
		op(registry)
	}

	return registry
}

// BasicAuth wraps a handler, allowing requests with matching username and password headers, otherwise rejecting with a 401
func BasicAuth(handler http.Handler, username, password, realm string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		if !ok || user != username || pass != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			_, _ = w.Write([]byte("Unauthorized.\n"))
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// ReadOnly wraps a handler, allowing only GET and HEAD requests, otherwise rejecting with a 405
func ReadOnly(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(r.Method == "GET" || r.Method == "HEAD") {
			w.WriteHeader(405)
			_, _ = w.Write([]byte("Method Not Allowed.\n"))
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func Custom(handler http.Handler, permissions map[string]*ImagePrivileges) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if permission, ok := permissions[extractImageName(r.URL.Path)]; ok {
			if r.Method == "GET" || r.Method == "HEAD" {
				if !permission.readable {
					w.WriteHeader(401)
					_, _ = w.Write([]byte("Unauthorized.\n"))
					return
				}
			} else {
				if !permission.writeable {
					w.WriteHeader(401)
					_, _ = w.Write([]byte("Unauthorized.\n"))
					return
				}
			}
		}
		handler.ServeHTTP(w, r)
	})
}

func (r *DockerRegistry) Start(t *testing.T) {
	t.Helper()

	r.Host = DockerHostname(t)

	// create registry handler, if not re-using a shared one
	if r.regHandler == nil {
		// change to os.Stderr for verbose output
		logger := registry.Logger(log.New(ioutil.Discard, "registry ", log.Lshortfile))
		r.regHandler = registry.New(logger)
	}

	// wrap registry handler with authentication handler, defaulting to read-only
	r.authnHandler = ReadOnly(r.regHandler)
	if r.username != "" {
		if r.customPrivileges != nil {
			// wrap registry handler with custom handler
			r.regHandler = Custom(r.regHandler, r.customPrivileges)
		}
		r.authnHandler = BasicAuth(r.regHandler, r.username, r.password, "registry")
	}

	// listen on specific interface with random port, relying on authorization to prevent untrusted writes
	listenIP := "127.0.0.1"
	if r.Host != "localhost" {
		listenIP = r.Host
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(listenIP, "0"))
	AssertNil(t, err)

	r.server = &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: r.authnHandler},
	}

	r.server.Start()

	tcpAddr := r.server.Listener.Addr().(*net.TCPAddr)

	r.Port = strconv.Itoa(tcpAddr.Port)
	t.Logf("run registry on %s:%s", r.Host, r.Port)

	if r.username != "" {
		// Write Docker config and configure auth headers
		writeDockerConfig(t, r.DockerDirectory, r.Host, r.Port, r.encodedAuth())
	}
}

func (r *DockerRegistry) Stop(t *testing.T) {
	t.Helper()
	t.Log("stop registry")

	r.server.Close()
}

func (r *DockerRegistry) RepoName(name string) string {
	return r.Host + ":" + r.Port + "/" + name
}

func (r *DockerRegistry) EncodedLabeledAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"username":"%s","password":"%s"}`, r.username, r.password)))
}

//DockerHostname discovers the appropriate registry hostname.
//For test to run where "localhost" is not the daemon host, a `insecure-registries` entry of `<host net>/<mask>` with a range that contains the host's non-loopback IP.
//For Docker Desktop, this can be set here: https://docs.docker.com/docker-for-mac/#docker-engine
//Otherwise, its set in the daemon.json: https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file
//If the entry is not found, the fallback is "localhost"
func DockerHostname(t *testing.T) string {
	dockerCli := DockerCli(t)

	// query daemon for insecure-registry network ranges
	var insecureRegistryNets []*net.IPNet
	daemonInfo, err := dockerCli.Info(context.TODO())
	if err != nil {
		t.Fatalf("unable to fetch client.DockerInfo: %s", err)
	}
	for _, ipnet := range daemonInfo.RegistryConfig.InsecureRegistryCIDRs {
		insecureRegistryNets = append(insecureRegistryNets, (*net.IPNet)(ipnet))
	}

	// search for non-loopback interface IPs contained by a insecure-registry range
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("unable to fetch interfaces: %s", err)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			t.Fatalf("unable to fetch interface address: %s", err)
		}

		for _, addr := range addrs {
			var interfaceIP net.IP
			switch interfaceAddr := addr.(type) {
			case *net.IPAddr:
				interfaceIP = interfaceAddr.IP
			case *net.IPNet:
				interfaceIP = interfaceAddr.IP
			}

			// ignore blanks and loopbacks
			if interfaceIP == nil || interfaceIP.IsLoopback() {
				continue
			}

			// return first matching interface IP
			for _, regNet := range insecureRegistryNets {
				if regNet.Contains(interfaceIP) {
					return interfaceIP.String()
				}
			}
		}
	}

	// Fallback to localhost, only works for Linux using --network=host
	return "localhost"
}

func (r *DockerRegistry) encodedAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", r.username, r.password)))
}

func writeDockerConfig(t *testing.T, configDir, host, port, auth string) {
	AssertNil(t, ioutil.WriteFile(
		filepath.Join(configDir, "config.json"),
		[]byte(fmt.Sprintf(`{
			  "auths": {
			    "%s:%s": {
			      "auth": "%s"
			    }
			  }
			}
			`, host, port, auth)),
		0666,
	))
}
