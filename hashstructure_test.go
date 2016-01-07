package hashstructure

import (
	"testing"
)

func TestHash_identity(t *testing.T) {
	cases := []interface{}{
		nil,
		"foo",
		42,
		true,
		false,
		[]string{"foo", "bar"},
		[]interface{}{1, nil, "foo"},
		map[string]string{"foo": "bar"},
		map[interface{}]string{"foo": "bar"},
		map[interface{}]interface{}{"foo": "bar", "bar": 0},
		struct {
			Foo string
			Bar []interface{}
		}{
			Foo: "foo",
			Bar: []interface{}{nil, nil, nil},
		},
		&struct {
			Foo string
			Bar []interface{}
		}{
			Foo: "foo",
			Bar: []interface{}{nil, nil, nil},
		},
	}

	for _, tc := range cases {
		// We run the test 100 times to try to tease out variability
		// in the runtime in terms of ordering.
		valuelist := make([]uint64, 100)
		for i, _ := range valuelist {
			v, err := Hash(tc, nil)
			if err != nil {
				t.Fatalf("Error: %s\n\n%#v", err, tc)
			}

			valuelist[i] = v
		}

		// Zero is always wrong
		if valuelist[0] == 0 {
			t.Fatalf("zero hash: %#v", tc)
		}

		// Make sure all the values match
		t.Logf("%#v: %d", tc, valuelist[0])
		for i := 1; i < len(valuelist); i++ {
			if valuelist[i] != valuelist[0] {
				t.Fatalf("non-matching: %d, %d\n\n%#v", i, 0, tc)
			}
		}
	}
}

func TestHash_equal(t *testing.T) {
	cases := []struct {
		One, Two interface{}
		Match    bool
	}{
		{
			map[string]string{"foo": "bar"},
			map[interface{}]string{"foo": "bar"},
			true,
		},
	}

	for _, tc := range cases {
		one, err := Hash(tc.One, nil)
		if err != nil {
			t.Fatalf("Failed to hash %#v: %s", tc.One, err)
		}
		two, err := Hash(tc.Two, nil)
		if err != nil {
			t.Fatalf("Failed to hash %#v: %s", tc.Two, err)
		}

		// Zero is always wrong
		if one == 0 {
			t.Fatalf("zero hash: %#v", tc.One)
		}

		// Compare
		if (one == two) != tc.Match {
			t.Fatalf("bad, expected: %#v\n\n%#v\n\n%#v", tc.Match, tc.One, tc.Two)
		}
	}
}
