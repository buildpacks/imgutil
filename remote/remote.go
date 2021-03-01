package remote

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/buildpacks/imgutil/layer"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"

	"github.com/buildpacks/imgutil"
)

type Image struct {
	keychain   authn.Keychain
	repoName   string
	platform   imgutil.Platform
	image      v1.Image
	prevLayers []v1.Layer
}

type ImageOption func(*Image) (*Image, error)
type InitialImageOption ImageOption

func WithPreviousImage(imageName string) ImageOption {
	return func(r *Image) (*Image, error) {
		var err error

		prevImage, err := newV1Image(r.keychain, imageName, r.platform)
		if err != nil {
			return nil, err
		}

		prevLayers, err := prevImage.Layers()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get layers for previous image with repo name '%s'", imageName)
		}

		r.prevLayers = prevLayers
		return r, nil
	}
}

func FromBaseImage(imageName string) ImageOption {
	return func(r *Image) (*Image, error) {
		var err error

		r.image, err = newV1Image(r.keychain, imageName, r.platform)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
}

func WithPlatform(platform imgutil.Platform) InitialImageOption {
	return func(r *Image) (*Image, error) {
		configFile, err := r.image.ConfigFile()
		if err != nil {
			return nil, err
		}

		configFile.Architecture = platform.Architecture
		configFile.OS = platform.OS
		configFile.OSVersion = platform.OSVersion

		r.image, err = mutate.ConfigFile(r.image, configFile)
		if err != nil {
			return nil, err
		}

		r.platform = platform

		return r, nil
	}
}

func NewImage(repoName string, keychain authn.Keychain, ops ...interface{}) (*Image, error) {
	image, err := emptyImage(defaultPlatform())
	if err != nil {
		return nil, err
	}

	ri := &Image{
		keychain: keychain,
		repoName: repoName,
		image:    image,
		platform: defaultPlatform(),
	}

	ri, err = processImageOptions(ri, ops)
	if err != nil {
		return nil, err
	}

	ri, err = prepareImage(ri)
	if err != nil {
		return nil, err
	}

	return ri, nil
}

func processImageOptions(image *Image, ops []interface{}) (*Image, error) {
	sort.Slice(ops, func(i, _ int) bool {
		switch ops[i].(type) {
		case InitialImageOption:
			return true
		default:
			return false
		}
	})

	for _, op := range ops {
		var err error

		switch o := op.(type) {
		case InitialImageOption:
			image, err = o(image)
		case ImageOption:
			image, err = o(image)
		}
		if err != nil {
			return nil, err
		}
	}

	return image, nil
}

func prepareImage(ri *Image) (*Image, error) {
	imageConfigFile, err := ri.image.ConfigFile()
	if err != nil {
		return nil, err
	}

	if imageConfigFile.OS != "windows" || len(imageConfigFile.RootFS.DiffIDs) != 0 {
		return ri, nil
	}

	layerBytes, err := layer.WindowsBaseLayer()
	if err != nil {
		return nil, err
	}
	windowsBaseLayer, err := tarball.LayerFromReader(layerBytes)
	if err != nil {
		return nil, err
	}
	ri.image, err = mutate.AppendLayers(ri.image, windowsBaseLayer)
	if err != nil {
		return nil, err
	}

	return ri, nil
}

func newV1Image(keychain authn.Keychain, repoName string, platform imgutil.Platform) (v1.Image, error) {
	ref, auth, err := referenceForRepoName(keychain, repoName)
	if err != nil {
		return nil, err
	}

	v1Platform := v1.Platform{
		Architecture: platform.Architecture,
		OS:           platform.OS,
		OSVersion:    platform.OSVersion,
	}

	image, err := remote.Image(ref, remote.WithAuth(auth), remote.WithTransport(http.DefaultTransport), remote.WithPlatform(v1Platform))
	if err != nil {
		if transportErr, ok := err.(*transport.Error); ok && len(transportErr.Errors) > 0 {
			switch transportErr.StatusCode {
			case http.StatusNotFound, http.StatusUnauthorized:
				return emptyImage(platform)
			}
		}
		if strings.Contains(err.Error(), "no child with platform") {
			return emptyImage(platform)
		}
		return nil, fmt.Errorf("connect to repo store '%s': %s", repoName, err.Error())
	}

	return image, nil
}

func emptyImage(platform imgutil.Platform) (v1.Image, error) {
	cfg := &v1.ConfigFile{
		Architecture: platform.Architecture,
		OS:           platform.OS,
		OSVersion:    platform.OSVersion,
		RootFS: v1.RootFS{
			Type:    "layers",
			DiffIDs: []v1.Hash{},
		},
	}

	return mutate.ConfigFile(empty.Image, cfg)
}

func defaultPlatform() imgutil.Platform {
	return imgutil.Platform{
		OS:           "linux",
		Architecture: "amd64",
	}
}

func referenceForRepoName(keychain authn.Keychain, ref string) (name.Reference, authn.Authenticator, error) {
	var auth authn.Authenticator
	r, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		return nil, nil, err
	}

	auth, err = keychain.Resolve(r.Context().Registry)
	if err != nil {
		return nil, nil, err
	}
	return r, auth, nil
}

func (i *Image) Label(key string) (string, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil {
		return "", fmt.Errorf("failed to get config file for image '%s'", i.repoName)
	}
	labels := cfg.Config.Labels
	return labels[key], nil
}

func (i *Image) Labels() (map[string]string, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil {
		return nil, fmt.Errorf("failed to get config file for image '%s'", i.repoName)
	}
	return cfg.Config.Labels, nil
}

func (i *Image) Env(key string) (string, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil {
		return "", fmt.Errorf("failed to get config file for image '%s'", i.repoName)
	}
	for _, envVar := range cfg.Config.Env {
		parts := strings.Split(envVar, "=")
		if parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (i *Image) Entrypoint() ([]string, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil {
		return nil, fmt.Errorf("failed to get config file for image '%s'", i.repoName)
	}
	return cfg.Config.Entrypoint, nil
}

func (i *Image) OS() (string, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil || cfg.OS == "" {
		return "", fmt.Errorf("failed to get OS from config file for image '%s'", i.repoName)
	}
	return cfg.OS, nil
}

func (i *Image) OSVersion() (string, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil {
		return "", fmt.Errorf("failed to get OSVersion from config file for image '%s'", i.repoName)
	}
	return cfg.OSVersion, nil
}

func (i *Image) Architecture() (string, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil || cfg.Architecture == "" {
		return "", fmt.Errorf("failed to get Architecture from config file for image '%s'", i.repoName)
	}
	return cfg.Architecture, nil
}

func (i *Image) Rename(name string) {
	i.repoName = name
}

func (i *Image) Name() string {
	return i.repoName
}

func (i *Image) Found() bool {
	ref, auth, err := referenceForRepoName(i.keychain, i.repoName)
	if err != nil {
		return false
	}
	_, err = remote.Head(ref, remote.WithAuth(auth), remote.WithTransport(http.DefaultTransport))
	return err == nil
}

func (i *Image) Identifier() (imgutil.Identifier, error) {
	ref, err := name.ParseReference(i.repoName, name.WeakValidation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference for image '%s': %s", i.repoName, err)
	}

	hash, err := i.image.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get digest for image '%s': %s", i.repoName, err)
	}

	digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", ref.Context().Name(), hash.String()), name.WeakValidation)
	if err != nil {
		return nil, errors.Wrap(err, "creating digest reference")
	}

	return DigestIdentifier{
		Digest: digestRef,
	}, nil
}

func (i *Image) CreatedAt() (time.Time, error) {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get createdAt time for image '%s': %s", i.repoName, err)
	}
	return configFile.Created.UTC(), nil
}

func (i *Image) Rebase(baseTopLayer string, newBase imgutil.Image) error {
	newBaseRemote, ok := newBase.(*Image)
	if !ok {
		return errors.New("expected new base to be a remote image")
	}

	newImage, err := mutate.Rebase(i.image, &subImage{img: i.image, topDiffID: baseTopLayer}, newBaseRemote.image)
	if err != nil {
		return errors.Wrap(err, "rebase")
	}

	newImageConfig, err := newImage.ConfigFile()
	if err != nil {
		return err
	}

	newBaseRemoteConfig, err := newBaseRemote.image.ConfigFile()
	if err != nil {
		return err
	}

	newImageConfig.Architecture = newBaseRemoteConfig.Architecture
	newImageConfig.OS = newBaseRemoteConfig.OS
	newImageConfig.OSVersion = newBaseRemoteConfig.OSVersion

	newImage, err = mutate.ConfigFile(newImage, newImageConfig)
	if err != nil {
		return err
	}

	i.image = newImage
	return nil
}

func (i *Image) SetLabel(key, val string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[key] = val
	i.image, err = mutate.Config(i.image, config)
	return err
}

func (i *Image) RemoveLabel(key string) error {
	cfg, err := i.image.ConfigFile()
	if err != nil || cfg == nil {
		return fmt.Errorf("failed to get config file for image '%s'", i.repoName)
	}
	config := *cfg.Config.DeepCopy()
	delete(config.Labels, key)
	i.image, err = mutate.Config(i.image, config)
	return err
}

func (i *Image) SetEnv(key, val string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	ignoreCase := configFile.OS == "windows"
	for idx, e := range config.Env {
		parts := strings.Split(e, "=")
		foundKey := parts[0]
		searchKey := key
		if ignoreCase {
			foundKey = strings.ToUpper(foundKey)
			searchKey = strings.ToUpper(searchKey)
		}
		if foundKey == searchKey {
			config.Env[idx] = fmt.Sprintf("%s=%s", key, val)
			i.image, err = mutate.Config(i.image, config)
			if err != nil {
				return err
			}
			return nil
		}
	}
	config.Env = append(config.Env, fmt.Sprintf("%s=%s", key, val))
	i.image, err = mutate.Config(i.image, config)
	return err
}

func (i *Image) SetWorkingDir(dir string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.WorkingDir = dir
	i.image, err = mutate.Config(i.image, config)
	return err
}

func (i *Image) SetEntrypoint(ep ...string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.Entrypoint = ep
	i.image, err = mutate.Config(i.image, config)
	return err
}

func (i *Image) SetCmd(cmd ...string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.Cmd = cmd
	i.image, err = mutate.Config(i.image, config)
	return err
}

func (i *Image) SetOS(osVal string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	configFile.OS = osVal
	i.image, err = mutate.ConfigFile(i.image, configFile)
	return err
}

func (i *Image) SetOSVersion(osVersion string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	configFile.OSVersion = osVersion
	i.image, err = mutate.ConfigFile(i.image, configFile)
	return err
}

func (i *Image) SetArchitecture(architecture string) error {
	configFile, err := i.image.ConfigFile()
	if err != nil {
		return err
	}
	configFile.Architecture = architecture
	i.image, err = mutate.ConfigFile(i.image, configFile)
	return err
}

func (i *Image) TopLayer() (string, error) {
	all, err := i.image.Layers()
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", fmt.Errorf("image %s has no layers", i.Name())
	}
	topLayer := all[len(all)-1]
	hex, err := topLayer.DiffID()
	if err != nil {
		return "", err
	}
	return hex.String(), nil
}

func (i *Image) GetLayer(sha string) (io.ReadCloser, error) {
	layers, err := i.image.Layers()
	if err != nil {
		return nil, err
	}

	layer, err := findLayerWithSha(layers, sha)
	if err != nil {
		return nil, err
	}

	return layer.Uncompressed()
}

func (i *Image) AddLayer(path string) error {
	layer, err := tarball.LayerFromFile(path)
	if err != nil {
		return err
	}
	i.image, err = mutate.AppendLayers(i.image, layer)
	if err != nil {
		return errors.Wrap(err, "add layer")
	}
	return nil
}

func (i *Image) AddLayerWithDiffID(path, diffID string) error {
	// this is equivalent to AddLayer in the remote case
	// it exists to provide optimize performance for local images
	return i.AddLayer(path)
}

func (i *Image) ReuseLayer(sha string) error {
	layer, err := findLayerWithSha(i.prevLayers, sha)
	if err != nil {
		return err
	}
	i.image, err = mutate.AppendLayers(i.image, layer)
	return err
}

func findLayerWithSha(layers []v1.Layer, diffID string) (v1.Layer, error) {
	for _, layer := range layers {
		dID, err := layer.DiffID()
		if err != nil {
			return nil, errors.Wrap(err, "get diff ID for previous image layer")
		}
		if diffID == dID.String() {
			return layer, nil
		}
	}
	return nil, fmt.Errorf(`previous image did not have layer with diff id '%s'`, diffID)
}

func (i *Image) Save(additionalNames ...string) error {
	var err error

	allNames := append([]string{i.repoName}, additionalNames...)

	i.image, err = mutate.CreatedAt(i.image, v1.Time{Time: imgutil.NormalizedDateTime})
	if err != nil {
		return errors.Wrap(err, "set creation time")
	}

	cfg, err := i.image.ConfigFile()
	if err != nil {
		return errors.Wrap(err, "get image config")
	}
	cfg = cfg.DeepCopy()

	layers, err := i.image.Layers()
	if err != nil {
		return errors.Wrap(err, "get image layers")
	}
	cfg.History = make([]v1.History, len(layers))
	for i := range cfg.History {
		cfg.History[i] = v1.History{
			Created: v1.Time{Time: imgutil.NormalizedDateTime},
		}
	}

	cfg.DockerVersion = ""
	cfg.Container = ""
	i.image, err = mutate.ConfigFile(i.image, cfg)
	if err != nil {
		return errors.Wrap(err, "zeroing history")
	}

	var diagnostics []imgutil.SaveDiagnostic
	for _, n := range allNames {
		if err := i.doSave(n); err != nil {
			diagnostics = append(diagnostics, imgutil.SaveDiagnostic{ImageName: n, Cause: err})
		}
	}
	if len(diagnostics) > 0 {
		return imgutil.SaveError{Errors: diagnostics}
	}

	return nil
}

func (i *Image) doSave(imageName string) error {
	ref, auth, err := referenceForRepoName(i.keychain, imageName)
	if err != nil {
		return err
	}
	return remote.Write(ref, i.image, remote.WithAuth(auth))
}

func (i *Image) Delete() error {
	id, err := i.Identifier()
	if err != nil {
		return err
	}
	ref, auth, err := referenceForRepoName(i.keychain, id.String())
	if err != nil {
		return err
	}
	return remote.Delete(ref, remote.WithAuth(auth))
}

func (i *Image) ManifestSize() (int64, error) {
	return i.image.Size()
}

type subImage struct {
	img       v1.Image
	topDiffID string
}

func (si *subImage) Layers() ([]v1.Layer, error) {
	all, err := si.img.Layers()
	if err != nil {
		return nil, err
	}
	for i, l := range all {
		d, err := l.DiffID()
		if err != nil {
			return nil, err
		}
		if d.String() == si.topDiffID {
			return all[0 : i+1], nil
		}
	}
	return nil, errors.New("could not find base layer in image")
}
func (si *subImage) ConfigFile() (*v1.ConfigFile, error)     { return si.img.ConfigFile() }
func (si *subImage) BlobSet() (map[v1.Hash]struct{}, error)  { panic("Not Implemented") }
func (si *subImage) MediaType() (types.MediaType, error)     { panic("Not Implemented") }
func (si *subImage) ConfigName() (v1.Hash, error)            { panic("Not Implemented") }
func (si *subImage) RawConfigFile() ([]byte, error)          { panic("Not Implemented") }
func (si *subImage) Digest() (v1.Hash, error)                { panic("Not Implemented") }
func (si *subImage) Manifest() (*v1.Manifest, error)         { panic("Not Implemented") }
func (si *subImage) RawManifest() ([]byte, error)            { panic("Not Implemented") }
func (si *subImage) LayerByDigest(v1.Hash) (v1.Layer, error) { panic("Not Implemented") }
func (si *subImage) LayerByDiffID(v1.Hash) (v1.Layer, error) { panic("Not Implemented") }
func (si *subImage) Size() (int64, error)                    { panic("Not Implemented") }
