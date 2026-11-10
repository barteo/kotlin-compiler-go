package syntax

import (
	"fmt"
	"reflect"
	"strings"
)

// Equal reports whether a and b, two ASTs from different front ends, have
// the same structure, ignoring positions if ignorePos is set.
func Equal(a, b *File, ignorePos bool) bool { return Diff(a, b, ignorePos) == "" }

// Diff returns a description of the first difference between a and b, or
// "" if they are equal. It compares every field, unexported ones too, and
// follows pointers once: a pointer seen before must correspond to the same
// pointer on the other side, so shared and cyclic nodes compare as such.
// Nil and empty slices and maps are equal. Map keys that are pointers, as
// in File.Annotated, are matched through that correspondence.
func Diff(a, b *File, ignorePos bool) string {
	d := &differ{ignorePos: ignorePos, seen: map[uintptr]uintptr{}, seenB: map[uintptr]uintptr{}}
	d.cmp("File", reflect.ValueOf(a), reflect.ValueOf(b))
	for _, m := range d.deferred {
		if d.diff != "" {
			break
		}
		d.cmpPtrMap(m.path, m.a, m.b)
	}
	return d.diff
}

var posType = reflect.TypeOf(Pos{})

type differ struct {
	ignorePos bool
	seen      map[uintptr]uintptr // pointer in a -> pointer in b
	seenB     map[uintptr]uintptr // pointer in b -> pointer in a
	deferred  []deferredMap
	diff      string
}

type deferredMap struct {
	path string
	a, b reflect.Value
}

func (d *differ) fail(path, format string, args ...any) {
	if d.diff == "" {
		d.diff = path + ": " + fmt.Sprintf(format, args...)
	}
}

func (d *differ) cmp(path string, a, b reflect.Value) {
	if d.diff != "" {
		return
	}
	if a.Type() != b.Type() {
		d.fail(path, "type %v vs %v", a.Type(), b.Type())
		return
	}
	if a.Type() == posType && d.ignorePos {
		return
	}
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				d.fail(path, "nil vs non-nil (%s vs %s)", describe(a), describe(b))
			}
			return
		}
		pa, pb := a.Pointer(), b.Pointer()
		if qb, ok := d.seen[pa]; ok {
			if qb != pb {
				d.fail(path, "shared node corresponds to different nodes")
			}
			return
		}
		if qa, ok := d.seenB[pb]; ok && qa != pa {
			d.fail(path, "shared node corresponds to different nodes")
			return
		}
		d.seen[pa], d.seenB[pb] = pb, pa
		d.cmp(path, a.Elem(), b.Elem())
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				d.fail(path, "nil vs non-nil (%s vs %s)", describe(a), describe(b))
			}
			return
		}
		d.cmp(path, a.Elem(), b.Elem())
	case reflect.Struct:
		t := a.Type()
		for i := 0; i < t.NumField(); i++ {
			d.cmp(path+"."+t.Field(i).Name, a.Field(i), b.Field(i))
		}
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			d.fail(path, "length %d vs %d", a.Len(), b.Len())
			return
		}
		for i := 0; i < a.Len(); i++ {
			d.cmp(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i))
		}
	case reflect.Map:
		if a.Len() != b.Len() {
			d.fail(path, "map length %d vs %d", a.Len(), b.Len())
			return
		}
		if k := a.Type().Key().Kind(); k == reflect.Interface || k == reflect.Pointer {
			d.deferred = append(d.deferred, deferredMap{path, a, b})
			return
		}
		for _, k := range a.MapKeys() {
			vb := b.MapIndex(k)
			if !vb.IsValid() {
				d.fail(path, "key %v missing", k)
				return
			}
			d.cmp(fmt.Sprintf("%s[%v]", path, k), a.MapIndex(k), vb)
		}
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if a.IsNil() != b.IsNil() {
			d.fail(path, "nil vs non-nil")
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			d.fail(path, "%v vs %v", a.Bool(), b.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			d.fail(path, "%v vs %v", a.Int(), b.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if a.Uint() != b.Uint() {
			d.fail(path, "%v vs %v", a.Uint(), b.Uint())
		}
	case reflect.Float32, reflect.Float64:
		if a.Float() != b.Float() {
			d.fail(path, "%v vs %v", a.Float(), b.Float())
		}
	case reflect.Complex64, reflect.Complex128:
		if a.Complex() != b.Complex() {
			d.fail(path, "%v vs %v", a.Complex(), b.Complex())
		}
	case reflect.String:
		if a.String() != b.String() {
			d.fail(path, "%q vs %q", a.String(), b.String())
		}
	default:
		d.fail(path, "cannot compare %v", a.Kind())
	}
}

// cmpPtrMap compares maps keyed by pointers to nodes, after the walk has
// matched the nodes.
func (d *differ) cmpPtrMap(path string, a, b reflect.Value) {
	for _, ka := range a.MapKeys() {
		k := ka
		if k.Kind() == reflect.Interface {
			k = k.Elem()
		}
		if k.Kind() != reflect.Pointer {
			d.fail(path, "key %v is not a pointer", k.Type())
			return
		}
		pb, ok := d.seen[k.Pointer()]
		if !ok {
			d.fail(path, "key %s is not in the tree", describe(k))
			return
		}
		var vb reflect.Value
		for _, kb := range b.MapKeys() {
			kk := kb
			if kk.Kind() == reflect.Interface {
				kk = kk.Elem()
			}
			if kk.Kind() == reflect.Pointer && kk.Pointer() == pb && kk.Type() == k.Type() {
				vb = b.MapIndex(kb)
				break
			}
		}
		if !vb.IsValid() {
			d.fail(path, "no entry for %s", describe(k))
			return
		}
		d.cmp(fmt.Sprintf("%s[%s]", path, describe(k)), a.MapIndex(ka), vb)
	}
}

// describe names a value for a difference: its type, and its position if
// it is a node.
func describe(v reflect.Value) string {
	if !v.IsValid() {
		return "invalid"
	}
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
		return "nil"
	}
	s := strings.TrimPrefix(v.Type().String(), "*syntax.")
	if !v.CanInterface() {
		return s
	}
	if n, ok := v.Interface().(interface{ Pos() Pos }); ok {
		p := n.Pos()
		s += fmt.Sprintf("@%d:%d", p.Line, p.Col)
	}
	return s
}
