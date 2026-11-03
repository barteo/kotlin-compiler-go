package types

import (
	"slices"
	"strings"
	"sync"
)

// A type's DSL markers are the annotation classes annotated @DslMarker
// that annotate it where it's written, (@M A).() -> Unit or foo<@M A>,
// or the typealias it's written with. They're kept through substitution,
// as kotlinc keeps a type's attributes: T := @M A makes (@N T) @M @N A.
// A Type holds them as M, an index into an interned table of the sets,
// so it stays three words and comparable; the table has room for 255
// sets, and a type with a further one keeps the ones it has.

var markerTab struct {
	sync.Mutex
	sets [][]*ClassSym // by M-1
	keys map[string]uint8
}

// Markers returns t's DSL markers.
func (t Type) Markers() []*ClassSym {
	if t.M == 0 {
		return nil
	}
	markerTab.Lock()
	defer markerTab.Unlock()
	return markerTab.sets[t.M-1]
}

// WithMarkers returns t with DSL markers ms added to its own.
func (t Type) WithMarkers(ms []*ClassSym) Type {
	if len(ms) == 0 || t.K == Invalid {
		return t
	}
	all := slices.Clone(t.Markers())
	for _, m := range ms {
		if !slices.Contains(all, m) {
			all = append(all, m)
		}
	}
	slices.SortFunc(all, func(a, b *ClassSym) int { return strings.Compare(a.Name, b.Name) })
	var key strings.Builder
	for _, m := range all {
		key.WriteString(m.Name)
		key.WriteByte(';')
	}
	markerTab.Lock()
	defer markerTab.Unlock()
	if i, ok := markerTab.keys[key.String()]; ok {
		t.M = i
		return t
	}
	if len(markerTab.sets) == 255 {
		return t
	}
	if markerTab.keys == nil {
		markerTab.keys = map[string]uint8{}
	}
	markerTab.sets = append(markerTab.sets, all)
	t.M = uint8(len(markerTab.sets))
	markerTab.keys[key.String()] = t.M
	return t
}
