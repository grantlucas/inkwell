package daydata

import (
	"fmt"
	"slices"
	"strings"
)

// Keys reads settings out of a widget's raw config, one call per key, the
// same way on every day widget: an absent key leaves its default, a set
// one is checked and stored, and every error names the widget and the
// key. The first bad key stops the rest, so a widget reads all of its
// keys in a row and checks Err once.
type Keys struct {
	widget string
	raw    map[string]any
	err    error
}

// ReadKeys reads widget's settings from raw.
func ReadKeys(widget string, raw map[string]any) *Keys {
	return &Keys{widget: widget, raw: raw}
}

// Err is the first bad key's error, or nil.
func (k *Keys) Err() error { return k.err }

// get returns key's value when it is set and no key before it failed.
func (k *Keys) get(key string) (any, bool) {
	if k.err != nil {
		return nil, false
	}
	v, ok := k.raw[key]
	return v, ok
}

// fail records the first bad key, naming the widget.
func (k *Keys) fail(format string, args ...any) {
	k.err = fmt.Errorf("%s: "+format, append([]any{k.widget}, args...)...)
}

// integer reads key as an int. YAML decodes 7 as an int and 7.5 as a
// float, so a float is a fraction, said back as written, and anything
// else is not a number at all.
func (k *Keys) integer(key string) (int, bool) {
	v, ok := k.get(key)
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		k.fail("%s must be a whole number, got %v", key, n)
	default:
		k.fail("%s must be an integer, got %T", key, v)
	}
	return 0, false
}

// Int reads key as a whole number from lo to hi into dst.
func (k *Keys) Int(key string, lo, hi int, dst *int) {
	n, ok := k.integer(key)
	if !ok {
		return
	}
	if n < lo || n > hi {
		k.fail("%s must be in [%d, %d], got %d", key, lo, hi, n)
		return
	}
	*dst = n
}

// Positive reads key as a whole number above zero into dst.
func (k *Keys) Positive(key string, dst *int) {
	n, ok := k.integer(key)
	if !ok {
		return
	}
	if n <= 0 {
		k.fail("%s must be positive, got %d", key, n)
		return
	}
	*dst = n
}

// Bool reads key as true or false into dst.
func (k *Keys) Bool(key string, dst *bool) {
	v, ok := k.get(key)
	if !ok {
		return
	}
	b, ok := v.(bool)
	if !ok {
		k.fail("%s must be a bool, got %T", key, v)
		return
	}
	*dst = b
}

// String reads key as a string into dst, and reports whether it was set:
// for a widget to which an empty string means something.
func (k *Keys) String(key string, dst *string) bool {
	v, ok := k.get(key)
	if !ok {
		return false
	}
	s, ok := v.(string)
	if !ok {
		k.fail("%s must be a string, got %T", key, v)
		return false
	}
	*dst = s
	return true
}

// Choice reads key as one of names into dst. It is a function rather
// than a method of Keys because Go methods can't take type parameters.
func Choice[T ~int](k *Keys, key string, names Names[T], dst *T) {
	var name string
	if !k.String(key, &name) {
		return
	}
	v, ok := names.Parse(name)
	if !ok {
		k.fail("%s must be one of %s, got %q", key, strings.Join(names, ", "), name)
		return
	}
	*dst = v
}

// Names are the config names of an enum's values, in value order from
// zero, as a widget's style is named in config.
type Names[T ~int] []string

// Name is v's config name.
func (n Names[T]) Name(v T) string { return n[v] }

// Parse returns the value name names, and whether it names one.
func (n Names[T]) Parse(name string) (T, bool) {
	i := slices.Index(n, name)
	return T(max(i, 0)), i >= 0
}

// List is every name in value order, a copy for an error or a test to
// print.
func (n Names[T]) List() []string { return slices.Clone(n) }
