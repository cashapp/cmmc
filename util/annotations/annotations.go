package annotations

import (
	"context"
	"strings"

	"github.com/cashapp/cmmc/util"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const listSep = ","

// Annotation is a string type that can be used as a
// convenience wrapper for the exported functions of this
// package.
type Annotation string

// String returns the annotation name.
func (a Annotation) String() string {
	return string(a)
}

// RemoveFromList removes the value from the list.
func (a Annotation) RemoveFromList(val string) UpdateFn {
	return RemoveFromList(string(a), val)
}

// AddToList adds the value to the annotation list.
func (a Annotation) AddToList(val string) UpdateFn {
	return AddToList(string(a), val)
}

// Add adds/sets a single value annotation.
func (a Annotation) Add(val string) UpdateFn {
	return AddToList(string(a), val)
}

// Remove removes the annotation.
func (a Annotation) Remove() UpdateFn {
	return Remove(string(a))
}

// ParseObjectName attempts to parse an object name from an annotation
// on the given object.
func (a Annotation) ParseObjectName(o client.Object) (types.NamespacedName, bool) {
	n, ok := o.GetAnnotations()[string(a)]
	if !ok {
		return types.NamespacedName{}, false
	}

	// N.B. This should be fully qualified so we don't specify the
	// namespace of the current resource as a default for NamespacedName.
	namespacedName, err := util.NamespacedName(n, "")
	if err != nil {
		return types.NamespacedName{}, false
	}

	return namespacedName, true
}

// UpdateFn is any function that mutates a string map.
type UpdateFn func(in map[string]string)

// Add returns an UpdateFn which adds a k/v map.
func Add(k, v string) UpdateFn {
	return func(in map[string]string) {
		in[k] = v
	}
}

// AddToList returns an UpdateFn which adds v to a list stored in k.
//
// If the list does not exist it will create it.
func AddToList(k, v string) UpdateFn {
	return func(in map[string]string) {
		if v == "" {
			return
		}

		current, ok := in[k]
		if !ok || current == "" {
			in[k] = v
			return
		}

		values := strings.Split(current, listSep)
		for _, val := range values {
			if v == val {
				return
			}
		}

		values = append(values, v)
		Add(k, strings.Join(values, listSep))(in)
	}
}

// RemoveFromList returns an UpdateFn which removes v from a list stored in k.
//
// If the list does not exist, it does nothing.
// If the value remaning afterwards is empty, it will delete the key.
func RemoveFromList(k, v string) UpdateFn {
	return func(in map[string]string) {
		current, ok := in[k]
		if !ok || current == "" {
			Remove(k)(in)
			return
		}

		var (
			values = strings.Split(current, listSep)
			out    = make([]string, len(values))
			i      = 0
		)

		for _, val := range values {
			if val != "" && val != v {
				out[i] = val
				i++
			}
		}

		out = out[:i]
		if len(out) == 0 {
			Remove(k)(in)
		} else {
			Add(k, strings.Join(out, listSep))(in)
		}
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
