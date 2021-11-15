package finalizer

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type (
	Context = context.Context
	Client  = client.Client
	Object  = client.Object
)

// Finalizer possibly executes a finalier on a resource.
type Finalizer interface {
	Execute(Context, Client, Object) (bool, error)
}

// New configures a finalizer given a name and a cleanup function.
//
// If there are many on a specific resource it may be useful to have
// them be a list/queue instead of a single one.
func New(n string, e ...func() error) Finalizer {
	return &finalizer{n, e}
}

func (f *finalizer) Execute(ctx Context, c Client, o Object) (bool, error) {
	// If we can add the finalizer we do, this means that
	// the resource is _not_ currently actively being deleted.
	if err := f.ensure(ctx, c, o); err != nil {
		return false, err
	}

	// If we are not currently attempting to delete this resource
	// we will _not_ be attempting to execute a finalizer, so we
	// stop now.
	if !isCurrentlyBeingDeleted(o) {
		return false, nil
	}

	// If the finalizer has already been removed, we are good
	// and we don't need to try to execute the finalizer.
	if !containsString(o.GetFinalizers(), f.Name) {
		return true, nil
	}

	// We attempt to execute the finalizer.
	for _, fn := range f.Exec {
		if fn != nil {
			if err := fn(); err != nil {
				return true, err
			}
		}
	}

	// We remove the finalizer once we are done.
	return true, f.remove(ctx, c, o)
}

type finalizer struct {
	Name string
	Exec []func() error
}

// ensure will make sure that a finalizer is set on the given object
// checking on whether we can add it in the first place.
func (f *finalizer) ensure(ctx Context, c Client, o Object) error {
	if !canAddFinalizer(o, f.Name) {
		return nil
	}

	controllerutil.AddFinalizer(o, f.Name)

	return c.Update(ctx, o) //nolint: wrapcheck
}

// remove removes the finalizer and updates the resource.
func (f *finalizer) remove(ctx Context, c Client, o Object) error {
	controllerutil.RemoveFinalizer(o, f.Name)

	return c.Update(ctx, o) //nolint: wrapcheck
}

func isCurrentlyBeingDeleted(o Object) bool {
	return !o.GetDeletionTimestamp().IsZero()
}

func canAddFinalizer(o Object, name string) bool {
	return !isCurrentlyBeingDeleted(o) && !containsString(o.GetFinalizers(), name)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}

	return false
}
