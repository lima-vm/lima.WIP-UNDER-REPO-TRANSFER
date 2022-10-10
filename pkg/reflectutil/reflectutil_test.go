package reflectutil

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/xorcare/pointer"
	"gotest.tools/v3/assert"
)

type Foo struct {
	BoolPtr, BoolPtr2, BoolPtr3, BoolPtr4 *bool
	IntPtr, IntPtr2, IntPtr3, IntPtr4     *int
	StrPtr, StrPtr2                       *string
	StrStrMap, StrStrMap2                 map[string]string
	StrSlice                              []string
	StrArray                              [2]string
	Struct                                FooChild
}

type FooChild struct {
	Int, Int2    int
	Str, Str2    string
	IP           net.IP
	IPSlice      []net.IP
	StrStrPtrMap map[string]*string
}

func TestMerge(t *testing.T) {
	d := &Foo{
		BoolPtr:  pointer.Bool(true),
		BoolPtr2: pointer.Bool(false),
		BoolPtr3: nil,
		BoolPtr4: pointer.Bool(true),
		IntPtr:   pointer.Int(42),
		IntPtr2:  pointer.Int(200),
		IntPtr3:  nil,
		IntPtr4:  pointer.Int(400),
		StrPtr:   pointer.String("hello"),
		StrPtr2:  pointer.String("world"),
		StrStrMap: map[string]string{
			"a": "apple",
			"b": "banana",
			"c": "cranberry",
		},
		StrStrMap2: nil,
		StrSlice:   []string{"alpha", "beta"},
		StrArray:   [2]string{"alabama", "alaska"},
		Struct: FooChild{
			Int:     -42,
			Int2:    -100,
			Str:     "bonjour",
			Str2:    "le monde",
			IP:      net.ParseIP("192.168.10.1"),
			IPSlice: []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2")},
			StrStrPtrMap: map[string]*string{
				"a": pointer.String("appricot"),
				"b": pointer.String("blueberry"),
				"c": nil,
				"d": pointer.String("dragonfruit"),
			},
		},
	}

	o := &Foo{
		BoolPtr:  pointer.Bool(false),
		BoolPtr2: pointer.Bool(true),
		BoolPtr3: pointer.Bool(true),
		BoolPtr4: nil,
		IntPtr:   pointer.Int(43),
		IntPtr2:  nil,
		IntPtr3:  pointer.Int(300),
		IntPtr4:  pointer.Int(0),
		StrPtr:   pointer.String("olleh"),
		StrPtr2:  nil,
		StrStrMap: map[string]string{
			"b": "beer",
			"c": "",
			"d": "daiquiri",
		},
		StrSlice: []string{"gamma", "delta"},
		StrStrMap2: map[string]string{
			"a": "america",
			"b": "brazil",
		},
		StrArray: [2]string{"california", "colorado"},
		Struct: FooChild{
			Int:     -43,
			Str:     "ruojnob",
			IP:      net.ParseIP("192.168.11.1"),
			IPSlice: []net.IP{net.ParseIP("10.0.0.3")},
			StrStrPtrMap: map[string]*string{
				"a": nil,
				"b": pointer.String("bacardi"),
				"c": pointer.String("cinzano"),
				"d": pointer.String(""),
			},
		},
	}

	expected := &Foo{
		BoolPtr:  pointer.Bool(false),     // overridden
		BoolPtr2: pointer.Bool(true),      // overridden
		BoolPtr3: pointer.Bool(true),      // overridden (d=nil)
		BoolPtr4: pointer.Bool(true),      // Not overridden (o=nil)
		IntPtr:   pointer.Int(43),         // overridden
		IntPtr2:  pointer.Int(200),        // Not overridden (o=nil)
		IntPtr3:  pointer.Int(300),        // overridden (d=nil)
		IntPtr4:  pointer.Int(0),          // overridden
		StrPtr:   pointer.String("olleh"), // overridden
		StrPtr2:  pointer.String("world"), // Not overridden (o=nil)
		StrStrMap: map[string]string{ // merged (d, o)
			"a": "apple",
			"b": "beer",
			"c": "",
			"d": "daiquiri",
		},
		StrStrMap2: map[string]string{ // merged (d=nil, o)
			"a": "america",
			"b": "brazil",
		},
		StrSlice: []string{"gamma", "delta", "alpha", "beta"}, // appended (o, d)
		StrArray: [2]string{"california", "colorado"},         // overridden
		Struct: FooChild{
			Int:     -43,                                                                                 // overridden
			Int2:    0,                                                                                   // overridden (o=zero)
			Str:     "ruojnob",                                                                           // overridden
			Str2:    "",                                                                                  // overridden (o=empty)
			IP:      net.ParseIP("192.168.11.1"),                                                         // overridden
			IPSlice: []net.IP{net.ParseIP("10.0.0.3"), net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2")}, // appended (o, d)
			StrStrPtrMap: map[string]*string{ // merged (d, o)
				"a": pointer.String("appricot"),
				"b": pointer.String("bacardi"),
				"c": pointer.String("cinzano"),
				"d": pointer.String(""),
			},
		},
	}

	x, err := Merge(d, o)
	assert.NilError(t, err)
	logX(t, d, "d")
	logX(t, o, "o")
	logX(t, x, "x")
	assert.DeepEqual(t, expected, x)
}

func logX(t testing.TB, x interface{}, format string, args ...interface{}) {
	// Print in JSON for human readability
	j, err := json.Marshal(x)
	assert.NilError(t, err)
	t.Logf(format+": %s", append(args, string(j))...)
}
