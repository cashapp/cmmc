package annotations

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateFn is any function that mutates a string map.
type UpdateFn func(in map[string]string)

// Add returns an UpdateFn which adds a k/v map.
func Add(k, v string) UpdateFn {
	return func(in map[string]string) {
		in[k] = v
	}
}

// Remove returns an UpdateFn which removes a key.
func Remove(k string) UpdateFn {
	return func(in map[string]string) {
		delete(in, k)
	}
}

// Set mutates the annotations on an object given UpdateFns.
func Set(o client.Object, fns ...UpdateFn) {
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	for _, f := range fns {
		f(annotations)
	}

	o.SetAnnotations(annotations)
}

// Apply applies UpdateFns to a resource.
func Apply(ctx context.Context, c client.Client, o client.Object, fns ...UpdateFn) error {
	Set(o, fns...)

	return errors.WithStack(c.Update(ctx, o))
}
