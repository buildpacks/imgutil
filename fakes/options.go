package fakes

import (
	"errors"
	"math/rand"

	"github.com/google/go-containerregistry/pkg/v1/types"
)

type IndexAddOption func(*IndexAddOptions) error
type IndexPushOption func(*IndexPushOptions) error

type IndexAddOptions struct {
	format types.MediaType
}
type IndexPushOptions struct{}

func WithFormat(format types.MediaType) IndexAddOption {
	return func(o *IndexAddOptions) error {
		if !format.IsImage() {
			return errors.New("unsupported format")
		}
		o.format = format
		return nil
	}
}

// Option is an optional parameter to the random functions
type Option func(opts *options)

type options struct {
	source    rand.Source
	withIndex bool

	// TODO opens the door to add this in the future
	// algorithm digest.Algorithm
}

func getOptions(opts []Option) *options {
	// get a random seed

	// TODO in go 1.20 this is fine (it will be random)
	seed := rand.Int63() //nolint:gosec
	/*
		// in prior go versions this needs to come from crypto/rand
		var b [8]byte
		_, err := crypto_rand.Read(b[:])
		if err != nil {
			panic("cryptographically secure random number generator is not working")
		}
		seed := int64(binary.LittleEndian.Int64(b[:]))
	*/

	// defaults
	o := &options{
		source: rand.NewSource(seed),
	}

	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithSource sets the random number generator source
func WithSource(source rand.Source) Option {
	return func(opts *options) {
		opts.source = source
	}
}

func WithIndex(withIndex bool) Option {
	return func(opts *options) {
		opts.withIndex = withIndex
	}
}
