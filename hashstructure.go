package hashstructure

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc64"
	"hash/fnv"
	"io"
	"reflect"
	"sort"
)

// HashOptions are options that are available for hashing.
type HashOptions struct {
	// Hasher is the hash function to use. If this isn't set, it will
	// default to CRC-64. CRC probably isn't the best hash function to use
	// but it is in the Go standard library and there is a lot of support
	// for hardware acceleration.
	Hasher hash.Hash64
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
		var err error

		// We first need to order the keys so it is a deterministic walk
		var hasher hash.Hash64
		m := make(map[uint64]reflect.Value)
		ks := make([]uint64, v.Len())
		keys := v.MapKeys()
		for i := 0; i < len(keys); i++ {
			k := keys[i]
			ks[i], err = Hash(k.Interface(), &HashOptions{Hasher: hasher})
			if err != nil {
				return err
			}

			// Hash collision! We use a secondary hash function. Reset
			// the loop and start over. If we already are trying a second
			// hash function, panic.
			if _, ok := m[ks[i]]; ok {
				if hasher != nil {
					return fmt.Errorf("unresolvable hash collision: %#v", k.Interface())
				}

				hasher = fnv.New64()
				i = 0
				continue
			}

			m[ks[i]] = k
		}

		// Go through the sorted keys and hash
		sort.Sort(uint64Slice(ks))
		for _, hashKey := range ks {
			k := m[hashKey]
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

// uint64Slice is a sortable uint64 slice
type uint64Slice []uint64

func (p uint64Slice) Len() int           { return len(p) }
func (p uint64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p uint64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
