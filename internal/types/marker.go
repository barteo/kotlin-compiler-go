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
// sets, and a type with a further one keeps the ones it has. The table
// is the process's, and a class is another symbol in each check, so a set
// holds the names of its classes.

var markerTab struct {
	sync.Mutex
	sets [][]string // by M-1, each sorted
	keys map[string]uint8
}

// Markers returns the names of t's DSL markers.
func (t Type) Markers() []string {
	if t.M == 0 {
		return nil
	}
	markerTab.Lock()
	defer markerTab.Unlock()
	return markerTab.sets[t.M-1]
}

// WithMarkers returns t with the DSL markers named ms added to its own.
func (t Type) WithMarkers(ms []string) Type {
	if len(ms) == 0 || t.K == Invalid {
		return t
	}
	all := slices.Clone(t.Markers())
	for _, m := range ms {
		if !slices.Contains(all, m) {
			all = append(all, m)
		}
	}
	slices.Sort(all)
	key := strings.Join(all, ";")
	markerTab.Lock()
	defer markerTab.Unlock()
	if i, ok := markerTab.keys[key]; ok {
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
	markerTab.keys[key] = t.M
	return t
}
