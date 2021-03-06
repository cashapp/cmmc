package annotations_test

import (
	"testing"

	. "github.com/cashapp/cmmc/util/annotations"
	"github.com/stretchr/testify/assert"
)

func TestAddToList(t *testing.T) {
	assertUpdated(
		t,
		map[string]string{},
		map[string]string{
			"foo": "bar,baz",
			"hey": "you",
		},
		AddToList("foo", "bar"), // add fo==bar
		AddToList("foo", "baz"), // add foo=baz (bar,baz)
		AddToList("hey", "you"), // add new key
		AddToList("foo", "bar"), // add dupliacte (noop)
		AddToList("foo", ""),    // noop
	)

	var (
		foo = Annotation("foo")
		hey = Annotation("hey")
	)

	assertUpdated(
		t,
		map[string]string{},
		map[string]string{
			"foo": "bar,baz",
			"hey": "you",
		},
		foo.AddToList("bar"),
		foo.AddToList("baz"),
		hey.AddToList("you"),
		foo.AddToList("bar"),
		foo.AddToList(""),
	)
}

func TestRemoveFromList(t *testing.T) {
	assertUpdated(
		t,
		map[string]string{
			"foo": "foo,bar,,,baz",
			"hey": "you",
		},
		map[string]string{
			"foo": "foo,baz",
		},
		RemoveFromList("foo", "bar"),     // removing bar
		RemoveFromList("foo", "bar"),     // removing bar (noop)
		RemoveFromList("hey", "you"),     // removing hey=you (drops the key)
		RemoveFromList("hey", "none"),    // noop
		RemoveFromList("missing", "key"), // noop
		RemoveFromList("foo", ""),        // noop
	)
}

func assertUpdated(t *testing.T, in, expected map[string]string, fns ...UpdateFn) {
	t.Helper()

	for _, f := range fns {
		f(in)
	}

	assert.Equal(t, expected, in)
}
