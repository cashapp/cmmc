package util

import (
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errEmptyName      = errors.New("name is empty")
	errEmptyNamespace = errors.New("default namespace is empty")
	errInvalidName    = errors.New("name is invalid")
)

const (
	separator = string(types.Separator)
)

// NamespacedName parses a resource name, and converts it into a types.NamespacedName.
//
// If the name looks like it has a namespace in it (meaning it has a separator)
// then we use that, otherwise we use the namespace that the resource is created in.
func NamespacedName(name, namespace string) (types.NamespacedName, error) {
	if name == "" {
		return types.NamespacedName{}, errors.WithStack(errEmptyName)
	}

	switch split := strings.Split(name, separator); len(split) {
	case 1:
		if namespace == "" {
			return types.NamespacedName{}, errors.WithStack(errEmptyNamespace)
		}
	case 2: //nolint: gomnd
		namespace = split[0]
		name = split[1]
	default:
		return types.NamespacedName{}, errors.Wrapf(errInvalidName, "%s is invalid", name)
	}

	return types.NamespacedName{Namespace: namespace, Name: name}, nil
}

// MustNamespacedName panics if it can't construct a types.NamespacedName.
func MustNamespacedName(name, namespace string) types.NamespacedName {
	n, err := NamespacedName(name, namespace)
	if err != nil {
		panic(err)
	}

	return n
}

// ObjectNamespacedName gets the types.NamespacedName from the k8s object.
func ObjectNamespacedName(o client.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: o.GetNamespace(),
		Name:      o.GetName(),
	}
}

// ObjectResourceName gets string representation of the k8s name.
func ObjectResourceName(o client.Object) string {
	return ObjectNamespacedName(o).String()
}
