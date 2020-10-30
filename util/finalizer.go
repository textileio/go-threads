package util

import (
	"context"
	"io"

	"github.com/hashicorp/go-multierror"
)

// Finalizer collects resources for convenient cleanup.
func NewFinalizer() *Finalizer {
	return &Finalizer{}
}

type Finalizer struct {
	resources []io.Closer
}

func (r *Finalizer) Add(cs ...io.Closer) {
	r.resources = append(r.resources, cs...)
}

func (r *Finalizer) Cleanup(err error) error {
	// release resources in a reverse order
	for i := len(r.resources) - 1; i >= 0; i-- {
		err = multierror.Append(r.resources[i].Close())
	}

	return err.(*multierror.Error).ErrorOrNil()
}

// Transform context cancellation function to be used with finalizer.
func NewContextCloser(cancel context.CancelFunc) io.Closer {
	return &ContextCloser{cf: cancel}
}

type ContextCloser struct {
	cf context.CancelFunc
}

func (cc ContextCloser) Close() error {
	cc.cf()
	return nil
}
