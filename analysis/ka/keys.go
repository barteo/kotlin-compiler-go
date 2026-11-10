package ka

import (
	"hash/maphash"
	"slices"
	"strings"
)

// fileKeys are the hashes, sorted, of what a file's model looks up and
// declares, which declEdits and dependents test the names a change
// declares against (declaredNames.hitKeys, declaredByKeys) without
// going through every file's lookups. A hash that collides can only add
// a file to those checked, never leave one out.
type fileKeys struct {
	lookups []uint64 // each lookup's scope and name
	scopes  []uint64 // each lookup's scope
	decls   []uint64 // each declaration's scope and name, as declaredBy walks them
}

var keySeed = maphash.MakeSeed()

// scopeKey hashes a scope.
func scopeKey(scope string) uint64 { return maphash.String(keySeed, scope) }

// nameKey hashes a name in a scope.
func nameKey(scope, name string) uint64 {
	var h maphash.Hash
	h.SetSeed(keySeed)
	h.WriteString(scope)
	h.WriteByte(0)
	h.WriteString(name)
	return h.Sum64()
}

// sortedHashes sorts ks and drops its duplicates.
func sortedHashes(ks []uint64) []uint64 {
	slices.Sort(ks)
	return slices.Clip(slices.Compact(ks))
}

// newFileKeys returns f's keys.
func newFileKeys(f *File) *fileKeys {
	k := &fileKeys{lookups: make([]uint64, 0, len(f.Lookups)), scopes: make([]uint64, 0, len(f.Lookups))}
	last := ""
	for i, l := range f.Lookups {
		k.lookups = append(k.lookups, nameKey(l.Scope, l.Name))
		// Lookups are sorted, so a scope's are mostly together.
		if i == 0 || l.Scope != last {
			k.scopes = append(k.scopes, scopeKey(l.Scope))
			last = l.Scope
		}
	}
	var walk func(scope string, ds []*Decl)
	walk = func(scope string, ds []*Decl) {
		for _, d := range ds {
			k.decls = append(k.decls, nameKey(scope, d.Name))
			if d.ClassID != "" {
				walk(strings.ReplaceAll(d.ClassID, "/", "."), d.Members)
			}
		}
	}
	walk(f.Package, f.Decls)
	k.lookups, k.scopes, k.decls = sortedHashes(k.lookups), sortedHashes(k.scopes), sortedHashes(k.decls)
	return k
}

// intersects reports whether sorted a and b share a key.
func intersects(a, b []uint64) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) == 0 {
		return false
	}
	// A few keys against many: search for each.
	if len(a)*8 < len(b) {
		for _, x := range a {
			if _, ok := slices.BinarySearch(b, x); ok {
				return true
			}
		}
		return false
	}
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			return true
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return false
}

// seal hashes n's names and classes, once they are all added, for
// hitKeys and declaredByKeys.
func (n *declaredNames) seal() {
	n.nameKeys = n.nameKeys[:0]
	for k := range n.names {
		n.nameKeys = append(n.nameKeys, nameKey(k[0], k[1]))
	}
	n.classKeys = n.classKeys[:0]
	for c := range n.classes {
		n.classKeys = append(n.classKeys, scopeKey(c))
	}
	n.nameKeys, n.classKeys = sortedHashes(n.nameKeys), sortedHashes(n.classKeys)
}

// hitKeys is hit for the file whose keys are k.
func (n *declaredNames) hitKeys(k *fileKeys) bool {
	return intersects(n.nameKeys, k.lookups) || intersects(n.classKeys, k.scopes)
}

// declaredByKeys is declaredBy for the file whose keys are k.
func (n *declaredNames) declaredByKeys(k *fileKeys) bool {
	return intersects(n.nameKeys, k.decls)
}

// affects reports whether f, whose keys are k, looks up one of n's names
// or declares one (hit, declaredBy).
func (n *declaredNames) affects(f *File, k *fileKeys) bool {
	hit := n.hitKeys(k) || n.declaredByKeys(k)
	if verifyKeys && !hit && (n.hit(f.Lookups) || n.declaredBy(f)) {
		panic("ka: fileKeys miss a name of " + f.Path)
	}
	return hit
}

// verifyKeys makes affects check its keys against the names, for tests.
var verifyKeys bool

// keysOf returns the keys of files, kept with the models while a check c
// keeps holds them.
func (c *checkCache) keysOf(files []*File) []*fileKeys {
	ks := make([]*fileKeys, len(files))
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys == nil {
		c.keys = map[*File]*fileKeys{}
	}
	for i, f := range files {
		k, ok := c.keys[f]
		if !ok {
			k = newFileKeys(f)
			c.keys[f] = k
		}
		ks[i] = k
	}
	return ks
}
