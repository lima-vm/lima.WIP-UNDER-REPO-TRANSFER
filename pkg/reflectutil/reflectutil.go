package reflectutil

import (
	"fmt"
	"net"
	"reflect"

	"github.com/huandu/go-clone"
)

// NonAppendableSliceTypes are non-appendable slice types, such as net.IP .
var NonAppendableSliceTypes = map[reflect.Type]struct{}{
	reflect.TypeOf(net.IP{}): {}, // []byte
}

// MergeMany merges vv and returns the new value.
// MergeMany does not alter vv.
//
// Maps are merged in "vv[0], vv[1], .., vv[N-1]" order.
// Slices are appended in the "vv[N-1], .., vv[1], vv[0]" order.
func MergeMany(vv ...interface{}) (interface{}, error) {
	if l := len(vv); l < 2 {
		return nil, fmt.Errorf("expected len(vv) >= 2, got %d", l)
	}
	x := vv[0]
	for _, v := range vv[1:] {
		var err error
		x, err = Merge(x, v)
		if err != nil {
			return x, err
		}
	}
	return x, nil
}

// Merge merges o (override) into d (default) and returns the new value.
// Merge does not alter o and d.
//
// Maps are merged in the "d, o" order.
// Slices are appended in the "o, d" order.
func Merge(d, o interface{}) (interface{}, error) {
	if o == nil {
		return d, nil
	}
	dVal, oVal := reflect.ValueOf(d), reflect.ValueOf(o)
	if dVal.Type() != oVal.Type() {
		return nil, fmt.Errorf("type mismatch: %T vs %T", d, o)
	}
	x := clone.Clone(d)
	xVal := reflect.ValueOf(x)
	merge(xVal, oVal)
	return x, nil
}

func merge(xVal, oVal reflect.Value) {
	switch k := xVal.Kind(); k {
	case reflect.Pointer:
		if !oVal.IsNil() {
			if xVal.IsNil() {
				xVal.Set(cloneVal(oVal))
			} else {
				merge(xVal.Elem(), oVal.Elem())
			}
		}
	case reflect.Struct:
		numField := xVal.NumField()
		for i := 0; i < numField; i++ {
			merge(xVal.Field(i), oVal.Field(i))
		}
	case reflect.Map:
		if xVal.IsNil() {
			xVal.Set(reflect.MakeMap(oVal.Type()))
		}
		oValIter := oVal.MapRange()
		for oValIter.Next() {
			k, v := oValIter.Key(), oValIter.Value()
			// Ignore nil pointer (but empty value like "" and 0 are not ignored)
			if v.Kind() == reflect.Pointer && v.IsNil() {
				continue
			}
			xVal.SetMapIndex(cloneVal(k), cloneVal(v))
		}
	case reflect.Array:
		xVal.Set(cloneVal(oVal))
	case reflect.Slice:
		if _, ok := NonAppendableSliceTypes[xVal.Type()]; ok {
			xVal.Set(cloneVal(oVal))
		} else {
			// o comes first
			xVal.Set(reflect.AppendSlice(cloneVal(oVal), xVal))
		}
	case reflect.Chan, reflect.Func, reflect.Interface:
		panic(fmt.Errorf("unexpected kind %+v", k))
	default:
		if xVal.CanSet() {
			// oVal is not a pointer(-ish), so no need to clone oVal
			xVal.Set(oVal)
		}
	}
}

func cloneVal(v reflect.Value) reflect.Value {
	return reflect.ValueOf(clone.Clone(v.Interface()))
}
