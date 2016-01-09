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
	// default to CRC-64. CRC probably isn't the best hash function to use
	// but it is in the Go standard library and there is a lot of support
	// for hardware acceleration.
	Hasher hash.Hash64

	// TagName is the struct tag to look at when hashing the structure.
	// By default this is "hash".
	TagName string
}

// Hash returns the hash value of an arbitrary value.
//
// If opts is nil, then default options will be used. See HashOptions
// for the default values.
//
// Notes on the value:
//
//   * Unexported fields on structs are ignored and do not affect the
//     hash value.
//
//   * Adding an exported field to a struct with the zero value will change
//     the hash value.
//
// For structs, the hashing can be controlled using tags. For example:
//
//    struct {
//        Name string
//        UUID string `hash:"ignore"`
//    }
//
// The available tag values are:
//
//   * "ignore" - The field will be ignored and not affect the hash code.
//
func Hash(v interface{}, opts *HashOptions) (uint64, error) {
	// Create default options
	if opts == nil {
		opts = &HashOptions{}
	}
	if opts.Hasher == nil {
		opts.Hasher = crc64.New(crc64.MakeTable(crc64.ECMA))
	}
	if opts.TagName == "" {
		opts.TagName = "hash"
	}

	// Reset the hash
	opts.Hasher.Reset()

	// Create our walker and walk the structure
	w := &walker{
		w:   opts.Hasher,
		tag: opts.TagName,
	}
	if err := w.visit(reflect.ValueOf(v)); err != nil {
		return 0, err
	}

	return opts.Hasher.Sum64(), nil
}

type walker struct {
	w   io.Writer
	tag string
}

func (w *walker) visit(v reflect.Value) error {
	// Loop since these can be wrapped in multiple layers of pointers
	// and interfaces.
	for {
		// If we have an interface, dereference it. We have to do this up
		// here because it might be a nil in there and the check below must
		// catch that.
		if v.Kind() == reflect.Interface {
			v = v.Elem()
			continue
		}

		if v.Kind() == reflect.Ptr {
			v = reflect.Indirect(v)
			continue
		}

		break
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
	case reflect.Bool:
		var tmp int8
		if v.Bool() {
			tmp = 1
		}
		v = reflect.ValueOf(tmp)
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
		// Build the hash for the map. We do this by XOR-ing all the key
		// and value hashes. This makes it deterministic despite ordering.
		var h uint64
		for _, k := range v.MapKeys() {
			v := v.MapIndex(k)

			kh, err := Hash(k.Interface(), nil)
			if err != nil {
				return err
			}
			vh, err := Hash(v.Interface(), nil)
			if err != nil {
				return err
			}

			h = h ^ kh ^ vh
		}

		return binary.Write(w.w, binary.LittleEndian, h)

	case reflect.Struct:
		t := v.Type()
		l := v.NumField()
		for i := 0; i < l; i++ {
			if v := v.Field(i); v.CanSet() || t.Field(i).Name != "_" {
				fieldType := t.Field(i)
				tag := fieldType.Tag.Get(w.tag)
				if tag == "ignore" {
					// Ignore this field
					continue
				}

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
