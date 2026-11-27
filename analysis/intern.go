package analysis

import (
	"encoding/binary"
	"unsafe"
)

// A typeTable hash-conses types: each distinct type is stored once, and
// the types it returns are shared and never changed. A type's Arguments
// hold its interned arguments' values, whose own slices are theirs, and
// its Supertypes are kept as given, never cloned.
type typeTable struct {
	m   map[typeKey]*Type
	ids map[*Type]uint32 // of the interned types, from 1
	buf []byte
}

// typeKey is what makes interned types the same: every field of Type,
// its arguments by their IDs and its supertypes by their slice's
// identity (types that differ only there are kept apart).
type typeKey struct {
	name, projection string
	flags            uint16
	owner            ElementID
	args             string
	supers           *string
	nsupers          int
}

const (
	tNullable = 1 << iota
	tMarkedNullable
	tPlatform
	tParameter
	tSAM
	tSuspend
	tOwner
	tArgs   // Arguments isn't nil
	tSupers // Supertypes isn't nil
)

func newTypeTable() *typeTable {
	return &typeTable{m: map[typeKey]*Type{}, ids: map[*Type]uint32{}}
}

// make returns the interned type of base, which has no Arguments, with
// args as its arguments: interned types, or nil for none.
func (tt *typeTable) make(base Type, args []*Type) *Type {
	k := typeKey{name: base.Name, projection: base.Projection, supers: unsafe.SliceData(base.Supertypes), nsupers: len(base.Supertypes)}
	for i, b := range []bool{base.Nullable, base.MarkedNullable, base.Platform, base.Parameter, base.SAM, base.Suspend,
		base.ParameterOwner != nil, args != nil, base.Supertypes != nil} {
		if b {
			k.flags |= 1 << i
		}
	}
	if base.ParameterOwner != nil {
		k.owner = *base.ParameterOwner
	}
	if len(args) > 0 {
		tt.buf = tt.buf[:0]
		for _, a := range args {
			tt.buf = binary.LittleEndian.AppendUint32(tt.buf, tt.ids[a])
		}
		if t, ok := tt.m[typeKey{name: k.name, projection: k.projection, flags: k.flags, owner: k.owner, args: unsafe.String(unsafe.SliceData(tt.buf), len(tt.buf)), supers: k.supers, nsupers: k.nsupers}]; ok {
			return t
		}
		k.args = string(tt.buf)
	} else if t, ok := tt.m[k]; ok {
		return t
	}
	t := new(Type)
	*t = base
	t.Arguments = nil
	if args != nil {
		t.Arguments = make([]Type, len(args))
		for i, a := range args {
			t.Arguments[i] = *a
		}
	}
	if base.ParameterOwner != nil {
		o := *base.ParameterOwner
		t.ParameterOwner = &o
	}
	tt.m[k] = t
	tt.ids[t] = uint32(len(tt.ids) + 1)
	return t
}

// value returns the interned type equal to t.
func (tt *typeTable) value(t *Type) *Type {
	var args []*Type
	if t.Arguments != nil {
		args = make([]*Type, len(t.Arguments))
		for i := range t.Arguments {
			args[i] = tt.value(&t.Arguments[i])
		}
	}
	return tt.make(*t, args)
}

// canon returns the interned type equal to t, which may be one; nil for
// nil.
func (tt *typeTable) canon(t *Type) *Type {
	if t == nil {
		return nil
	}
	if _, ok := tt.ids[t]; ok {
		return t
	}
	return tt.value(t)
}
