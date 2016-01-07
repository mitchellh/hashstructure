package hashstructure

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc64"
	"io"
	"reflect"
)

// HashOptions are options that are available for hashing.
type HashOptions struct {
	// Hasher is the hash function to use. If this isn't set, it will
	// default to CRC-64.
	Hasher hash.Hash64
}

// Hash returns the hash value of an arbitrary value.
//
// If opts is nil, then default options will be used. See HashOptions
// for the default values.
func Hash(v interface{}, opts *HashOptions) (uint64, error) {
	// Create default options
	if opts == nil {
		opts = &HashOptions{}
	}
	if opts.Hasher == nil {
		opts.Hasher = crc64.New(crc64.MakeTable(crc64.ECMA))
	}

	// Reset the hash
	opts.Hasher.Reset()

	// Create our walker and walk the structure
	w := &walker{w: opts.Hasher}
	if err := w.visit(reflect.ValueOf(v)); err != nil {
		return 0, err
	}

	return opts.Hasher.Sum64(), nil
}

type walker struct {
	w io.Writer
}

func (w *walker) visit(v reflect.Value) error {
	// If we have an interface, dereference it. We have to do this up
	// here because it might be a nil in there and the check below must
	// catch that.
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	// If it is nil, treat it like a zero.
	if !v.IsValid() {
		var tmp int8
		v = reflect.ValueOf(tmp)
	}

	// Binary writing can use raw ints, we have to convert to
	// a sized-int, we'll choose the largest...
	switch v.Kind() {
	case reflect.Int:
		v = reflect.ValueOf(int64(v.Int()))
	case reflect.Uint:
		v = reflect.ValueOf(uint64(v.Uint()))
	}

	k := v.Kind()

	// We can shortcut numeric values by directly binary writing them
	if k >= reflect.Int && k <= reflect.Complex64 {
		return binary.Write(w.w, binary.LittleEndian, v.Interface())
	}

	switch k {
	case reflect.Array:
		l := v.Len()
		for i := 0; i < l; i++ {
			if err := w.visit(v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Map:
		for _, k := range v.MapKeys() {
			v := v.MapIndex(k)
			if err := w.visit(k); err != nil {
				return err
			}
			if err := w.visit(v); err != nil {
				return err
			}
		}

	case reflect.Struct:
		t := v.Type()
		l := v.NumField()
		for i := 0; i < l; i++ {
			if v := v.Field(i); v.CanSet() || t.Field(i).Name != "_" {
				if err := w.visit(v); err != nil {
					return err
				}
			}
		}

	case reflect.Slice:
		l := v.Len()
		for i := 0; i < l; i++ {
			if err := w.visit(v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.String:
		_, err := w.w.Write([]byte(v.String()))
		return err

	default:
		return fmt.Errorf("unknown kind to hash: %s", k)
	}

	return nil
}
