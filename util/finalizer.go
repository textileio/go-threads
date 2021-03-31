package util

import (
	"context"
	"io"

	"github.com/hashicorp/go-multierror"
)

// NewFinalizer returns a new Finalizer.
func NewFinalizer() *Finalizer {
	return &Finalizer{}
}

// Finalizer collects resources for convenient cleanup.
type Finalizer struct {
	resources []io.Closer
}

func (r *Finalizer) Add(cs ...io.Closer) {
	r.resources = append(r.resources, cs...)
}

func (r *Finalizer) Cleanup(err error) error {
	var errs []error
	for i := len(r.resources) - 1; i >= 0; i-- {
		// release resources in a reverse order
		if e := r.resources[i].Close(); e != nil {
			errs = append(errs, e)
		}
	}
	return multierror.Append(err, errs...).ErrorOrNil()
}

// NewContextCloser transforms context cancellation function to be used with finalizer.
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
