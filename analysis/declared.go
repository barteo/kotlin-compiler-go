package analysis

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/kasession"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// What a declaration edit affects. A file's checker reports the names it
// looks up in packages and classes (check.LookupRecorder), a superset of
// what its facts and errors depend on in other files. An update that
// changes what a file declares checks again the files that looked up a
// name it declared before or after, or that declare one themselves
// (overloads and conflicts), with the edited files; the others keep their
// facts, as for a body edit.

// A lookup is a name looked up in a package or class scope: a.b, or
// a.b.C for a member or nested class of C.
type lookup struct{ scope, name string }

// declInfo is what a file declares: each name, with the hash of the
// top-level declaration that declares it, and the supertypes' names of
// its top-level classes.
type declInfo struct {
	names map[lookup]uint64
	// supers are the simple names a top-level class's supertypes are
	// written with, by the class's name: a class added to a sealed
	// hierarchy changes the checks of its supertypes' uses.
	supers map[lookup][]string
}

// declared returns what snap declares.
func declared(snap *psi.Snapshot) declInfo {
	d := declInfo{names: map[lookup]uint64{}, supers: map[lookup][]string{}}
	root := snap.RootNode()
	pkg := root.ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
	for c := root.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindFUN, psi.KindPROPERTY, psi.KindTYPEALIAS:
		default:
			continue
		}
		h := kasession.DeclarationHash(c)
		d.add(pkg, c, h)
		if k := c.NumericKind(); k == psi.KindCLASS || k == psi.KindOBJECT_DECLARATION {
			key := lookup{pkg, c.Name()}
			psi.WalkNodes(c.ChildKind(psi.KindSUPER_TYPE_LIST), func(n psi.NodeRef) bool {
				if n.NumericKind() == psi.KindREFERENCE_EXPRESSION {
					d.supers[key] = append(d.supers[key], n.Name())
				}
				return true
			})
		}
	}
	return d
}

// add records the names declaration n declares in scope, with h, its
// top-level declaration's hash.
func (d declInfo) add(scope string, n psi.NodeRef, h uint64) {
	name := n.Name()
	if name == "" {
		return
	}
	k := lookup{scope, name}
	d.names[k] += h // overloads: the sum of their declarations'
	switch n.NumericKind() {
	case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
	default:
		return
	}
	inner := name
	if scope != "" {
		inner = scope + "." + name
	}
	companion := n.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindCOMPANION_KEYWORD).Valid()
	for _, p := range n.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).Parameters() {
		if p.ChildKind(psi.KindVAL_KEYWORD).Valid() || p.ChildKind(psi.KindVAR_KEYWORD).Valid() {
			d.add(inner, p, h)
		}
	}
	for m := n.ChildKind(psi.KindCLASS_BODY).FirstChild(); m.Valid(); m = m.NextSibling() {
		switch m.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindFUN, psi.KindPROPERTY, psi.KindTYPEALIAS, psi.KindENUM_ENTRY:
			d.add(inner, m, h)
			if companion {
				d.add(scope, m, h) // Outer.name may name a companion's member
			}
		}
	}
}

// touched adds to names the names old and cur declare differently, and to
// supers the supertypes' names of the top-level classes among them.
func touched(old, cur declInfo, names map[lookup]bool, supers map[string]bool) {
	mark := func(k lookup) {
		names[k] = true
		for _, s := range old.supers[k] {
			supers[s] = true
		}
		for _, s := range cur.supers[k] {
			supers[s] = true
		}
	}
	for k, h := range old.names {
		if cur.names[k] != h {
			mark(k)
		}
	}
	for k := range cur.names {
		if _, ok := old.names[k]; !ok {
			mark(k)
		}
	}
}

// affected reports whether a file that looked up lookups (sorted) and
// declares decl depends on the touched names.
func affected(lookups []lookup, decl declInfo, names map[lookup]bool, supers map[string]bool) bool {
	for k := range names {
		if _, ok := decl.names[k]; ok {
			return true
		}
		if _, ok := slices.BinarySearchFunc(lookups, k, compareLookup); ok {
			return true
		}
	}
	if len(supers) > 0 {
		for _, l := range lookups {
			if supers[l.name] {
				return true
			}
		}
	}
	return false
}

func compareLookup(a, b lookup) int {
	if c := strings.Compare(a.scope, b.scope); c != 0 {
		return c
	}
	return strings.Compare(a.name, b.name)
}

// Lookup implements check.LookupRecorder.
func (r *recorder) Lookup(f *syntax.File, scope string, _ check.LookupKind, name string) {
	r.mu.Lock()
	if r.lookups == nil {
		r.lookups = map[*syntax.File][]lookup{}
	}
	r.lookups[f] = append(r.lookups[f], lookup{scope, name})
	r.mu.Unlock()
}

// fileLookups returns f's lookups, sorted and without duplicates.
func (r *recorder) fileLookups(f *syntax.File) []lookup {
	r.mu.Lock()
	ls := slices.Clone(r.lookups[f])
	r.mu.Unlock()
	slices.SortFunc(ls, compareLookup)
	return slices.Compact(ls)
}
