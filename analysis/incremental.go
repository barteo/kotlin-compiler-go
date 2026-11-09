package analysis

import (
	"context"
	"slices"
	"sync/atomic"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/kasession"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Incremental updates. An editor changes one file at a time, and mostly
// a body: an update whose changed files declare what they did
// (kasession.SignatureHash) checks only those files (check.Options.Only),
// builds their facts, and keeps the others' facts and diagnostics from
// the last update. What a file's facts and errors say depends on its own
// text and on what the other files declare, not on their bodies, so the
// others' are still right. Every other update is checked whole.
//
// Kept facts and symbols keep their numbers, and are shared with the last
// update's Result unless they refer to a changed file: what they refer to
// in its last parse is mapped to its new parse, whose nodes outside block
// bodies correspond one to one (kasession.SignatureNodes). The changed
// files' facts name the kept symbols where they name the same
// declaration in the sources, or the same symbol of the library as the
// last update's own facts did; other symbols are added. A library's
// symbol that only kept facts name is described as the update that found
// it did, and an update's own facts may name it again under another
// number. So a Result's symbols may include some no fact names, until
// there are too many of them (state.compact).

// incrementalUpdates counts the updates that check only some files, for
// tests.
var incrementalUpdates atomic.Int64

// compactions counts the incremental updates that number their symbols
// again, for tests.
var compactions atomic.Int64

// state is what an incremental update starts from: the last update's
// snapshots, and each file's facts and diagnostics.
type state struct {
	res   *Result // its files are each file's facts
	snaps []*psi.Snapshot
	diags [][]Diagnostic
	// refs are the other snapshots each file's facts refer to.
	refs [][]psi.SnapshotID
	// syms are the symbols of res.symbols as the checker made them, or nil
	// for one an earlier update found.
	syms []any
	// base is how many symbols res had when they were last numbered
	// whole: an incremental update keeps the numbers of the symbols it
	// keeps, and adds the changed files' new ones, until there are too
	// many (compact).
	base int
}

// newState returns the state of res, an update of snaps whose every error
// is one file's, and syms its symbols; nil if two snapshots have the same
// path.
func newState(res *Result, snaps []*psi.Snapshot, syms []any) *state {
	if res.files == nil {
		res.setFiles(snaps, make([][]*Fact, len(snaps)))
	}
	st := &state{res: res, snaps: snaps, diags: make([][]Diagnostic, len(snaps)), refs: make([][]psi.SnapshotID, len(snaps)),
		syms: make([]any, len(res.symbols))}
	copy(st.syms, syms)
	st.base = len(res.symbols)
	byPath := map[string]int{}
	for i, s := range snaps {
		if _, dup := byPath[s.Path()]; dup {
			return nil
		}
		byPath[s.Path()] = i
		st.refs[i] = fileRefs(s.ID(), res.files[i])
	}
	for _, d := range res.diags {
		i, ok := byPath[d.Path]
		if !ok {
			return nil
		}
		st.diags[i] = append(st.diags[i], d)
	}
	return st
}

// fileRefs returns the snapshots but own that facts refer to.
func fileRefs(own psi.SnapshotID, facts []*Fact) []psi.SnapshotID {
	var refs []psi.SnapshotID
	for _, f := range facts {
		factIDs(f, func(id *ElementID) {
			if id.Snapshot != own && !slices.Contains(refs, id.Snapshot) {
				refs = append(refs, id.Snapshot)
			}
		})
	}
	return refs
}

// nodeMap maps the nodes outside block bodies of a changed file's last
// parse to its new one's.
type nodeMap struct {
	snap     psi.SnapshotID // the new parse's
	old, cur []psi.NodeID
}

// changedFiles returns the indexes of the snapshots that differ from
// st's, and the maps of their nodes, if only bodies changed; ok is false
// otherwise.
func (st *state) changedFiles(snaps []*psi.Snapshot) (changed []int, maps map[psi.SnapshotID]*nodeMap, ok bool) {
	if len(snaps) != len(st.snaps) {
		return nil, nil, false
	}
	maps = map[psi.SnapshotID]*nodeMap{}
	for i, s := range snaps {
		old := st.snaps[i]
		if s.Path() != old.Path() {
			return nil, nil, false
		}
		if s == old {
			continue
		}
		if kasession.SignatureHash(s) != kasession.SignatureHash(old) {
			return nil, nil, false
		}
		m := &nodeMap{snap: s.ID(), old: kasession.SignatureNodes(old), cur: kasession.SignatureNodes(s)}
		if len(m.old) != len(m.cur) {
			return nil, nil, false
		}
		for j := range m.old {
			if old.Node(m.old[j]).NumericKind() != s.Node(m.cur[j]).NumericKind() {
				return nil, nil, false
			}
		}
		maps[old.ID()] = m
		changed = append(changed, i)
	}
	return changed, maps, true
}

// remapper rewrites kept facts and symbols for a new update.
type remapper struct {
	nodes map[psi.SnapshotID]*nodeMap
	ok    bool // every element mapped
}

// element maps id, a node of a last parse, to the new parse's.
func (m *remapper) element(id *ElementID) {
	nm := m.nodes[id.Snapshot]
	if nm == nil {
		return
	}
	j, found := slices.BinarySearch(nm.old, id.Node)
	if !found {
		m.ok = false // in a body: an update must check it whole
		return
	}
	*id = ElementID{nm.snap, nm.cur[j]}
}

// incremental returns the update of snaps from st, checking only the
// changed files, and its state; ok is false if the update must be checked
// whole.
func incremental(ctx context.Context, cfg Config, snaps []*psi.Snapshot, st *state, open openLibrary) (res *Result, next *state, ok bool, err error) {
	changed, nodes, ok := st.changedFiles(snaps)
	if !ok {
		return nil, nil, false, nil
	}
	if len(changed) == 0 {
		return st.res, st, true, nil
	}
	isChanged := make([]bool, len(snaps))
	for _, i := range changed {
		isChanged[i] = true
	}
	// The kept symbols and facts, their elements mapped first: a kept fact
	// that names a symbol declared in a changed file's body means the
	// update must be checked whole. What doesn't refer to a changed file
	// is shared with the last update's Result, which doesn't change.
	m := &remapper{nodes: nodes}
	touches := func(walk func(func(*ElementID))) bool {
		hit := false
		walk(func(id *ElementID) {
			if nodes[id.Snapshot] != nil {
				hit = true
			}
		})
		return hit
	}
	syms := slices.Clone(st.res.symbols)
	dead := make([]bool, len(syms))
	for i := range syms {
		if !touches(func(fn func(*ElementID)) { symbolIDs(&syms[i], fn) }) {
			continue
		}
		s := copySymbol(syms[i])
		m.ok = true
		symbolIDs(&s, m.element)
		if !m.ok {
			dead[i] = true // a local of a changed body: no kept fact may name it
			continue
		}
		syms[i] = s
	}
	isDead := func(id uint32) bool { return id != 0 && dead[id-1] }
	m.ok = true
	n := len(snaps)
	kept := make([][]*Fact, n)
	refs := make([][]psi.SnapshotID, n)
	for i, fs := range st.res.files {
		if isChanged[i] {
			continue
		}
		kept[i], refs[i] = fs, st.refs[i]
		for _, f := range fs {
			if isDead(f.Symbol) || slices.ContainsFunc(f.LambdaParameters, isDead) {
				return nil, nil, false, nil
			}
		}
		if !slices.ContainsFunc(refs[i], func(id psi.SnapshotID) bool { return nodes[id] != nil }) {
			continue // shared
		}
		kept[i] = make([]*Fact, len(fs))
		for j, f := range fs {
			if touches(func(fn func(*ElementID)) { factIDs(f, fn) }) {
				c := copyFact(*f)
				factIDs(&c, m.element)
				f = &c
			}
			kept[i][j] = f
		}
		refs[i] = slices.Clone(refs[i])
		for k, id := range refs[i] {
			if nm := nodes[id]; nm != nil {
				refs[i][k] = nm.snap
			}
		}
	}
	if !m.ok {
		return nil, nil, false, nil
	}

	jobs := max(cfg.Jobs, 1)
	files := make([]*syntax.File, n)
	provs := make([]*lower.Provenance, n)
	errLists := make([][]*syntax.Error, n)
	parallel(n, jobs, func(i int) {
		if isChanged[i] {
			files[i], provs[i], errLists[i] = lower.Recover(snaps[i])
		} else {
			// Only declared: its facts and diagnostics are kept.
			files[i], provs[i], _ = lower.Signatures(snaps[i])
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, nil, false, err
	}
	diags := make([][]Diagnostic, n)
	broken := make([]bool, n)
	drops := make([][]span, n)
	for i := range snaps {
		if !isChanged[i] {
			diags[i] = st.diags[i]
			broken[i] = true // its facts are kept
			continue
		}
		for _, e := range errLists[i] {
			d := diagnostic(snaps[i], e)
			diags[i] = append(diags[i], d)
			if drop(snaps[i], d, &drops[i], &broken[i]) != reachCode {
				// Lowering dropped a declaration the other files may
				// use, as a whole update does.
				return nil, nil, false, nil
			}
		}
	}
	cp, lib, done, err := open()
	if err != nil {
		return nil, nil, false, err
	}
	defer done()
	only := map[*syntax.File]bool{}
	for _, i := range changed {
		only[files[i]] = true
	}
	rec := newRecorder()
	errs, err := check.CheckOptions(ctx, files, lib, jobs, rec, check.Options{Only: only})
	if err != nil {
		return nil, nil, false, err
	}
	index := map[string]int{}
	for i, s := range snaps {
		index[s.Path()] = i
	}
	for _, e := range errs {
		i, found := index[e.File]
		if !found || !isChanged[i] {
			return nil, nil, false, nil // not a changed file's
		}
		d := diagnostic(snaps[i], e)
		diags[i] = append(diags[i], d)
		drop(snaps[i], d, &drops[i], &broken[i])
	}

	// The changed files' facts, their symbols numbered into the kept
	// table: the same symbol of the library as the last update's facts
	// named, or the same declaration in the sources; else added.
	built := &Result{}
	var builtSyms []any
	if slices.Contains(broken, false) {
		b := newBuilder(built, snaps, files, provs, rec)
		b.cp = cp
		b.broken, b.drops = broken, drops
		b.build()
		builtSyms = b.syms
	}
	res = &Result{symbols: syms}
	next = &state{res: res, snaps: snaps, diags: diags, refs: refs, syms: make([]any, len(syms)), base: st.base}
	byPtr := map[any]uint32{}
	for i, p := range st.syms {
		if p != nil {
			byPtr[p] = uint32(i + 1)
		}
	}
	byDecl := map[declKey]uint32{}
	for i, s := range syms {
		if s.Declaration != nil && !dead[i] {
			byDecl[declKey{*s.Declaration, s.Kind, s.Name}] = s.ID
		}
	}
	ids := make([]uint32, len(built.symbols))
	for k, s := range built.symbols {
		p := builtSyms[k]
		var id uint32
		if p != nil {
			id = byPtr[p]
		}
		if id == 0 && s.Declaration != nil {
			id = byDecl[declKey{*s.Declaration, s.Kind, s.Name}]
		}
		if id == 0 {
			id = uint32(len(res.symbols) + 1)
			res.symbols = append(res.symbols, Symbol{})
			next.syms = append(next.syms, nil)
		}
		s.ID = id
		res.symbols[id-1] = s
		next.syms[id-1] = p
		ids[k] = id
	}
	renumber := func(id *uint32) {
		if *id != 0 {
			*id = ids[*id-1]
		}
	}
	all := make([][]*Fact, n)
	for i := range snaps {
		if !isChanged[i] {
			all[i] = kept[i]
		} else if built.files != nil {
			all[i] = built.files[i]
			for _, f := range all[i] {
				renumber(&f.Symbol)
				for k := range f.LambdaParameters {
					renumber(&f.LambdaParameters[k])
				}
			}
			refs[i] = fileRefs(snaps[i].ID(), all[i])
		}
		res.diags = append(res.diags, diags[i]...)
	}
	res.setFiles(snaps, all)
	if len(res.symbols) > 2*next.base+compactSlack {
		next.compact()
	}
	incrementalUpdates.Add(1)
	return res, next, true, nil
}

// compactSlack is how many symbols beyond twice the last numbering's an
// update may have before it compacts them.
var compactSlack = 256

type declKey struct {
	el         ElementID
	kind, name string
}

// compact numbers the symbols that facts name again, dropping the others:
// those only the facts of changed files' earlier versions named.
func (st *state) compact() {
	res := st.res
	ids := make([]uint32, len(res.symbols))
	mark := func(id uint32) {
		if id != 0 {
			ids[id-1] = 1
		}
	}
	for _, fs := range res.files {
		for _, f := range fs {
			mark(f.Symbol)
			for _, p := range f.LambdaParameters {
				mark(p)
			}
		}
	}
	var syms []Symbol
	var ptrs []any
	for i, used := range ids {
		if used == 0 {
			continue
		}
		ids[i] = uint32(len(syms) + 1)
		s := res.symbols[i]
		s.ID = ids[i]
		syms = append(syms, s)
		ptrs = append(ptrs, st.syms[i])
	}
	moved := func(id uint32) bool { return id != 0 && ids[id-1] != id }
	for i, fs := range res.files {
		cloned := false
		for j, f := range fs {
			if !moved(f.Symbol) && !slices.ContainsFunc(f.LambdaParameters, moved) {
				continue
			}
			if !cloned { // it may be an earlier Result's
				fs, cloned = slices.Clone(fs), true
				res.files[i] = fs
			}
			c := copyFact(*f)
			if c.Symbol != 0 {
				c.Symbol = ids[c.Symbol-1]
			}
			for k, p := range c.LambdaParameters {
				c.LambdaParameters[k] = ids[p-1]
			}
			fs[j] = &c
		}
	}
	res.symbols, st.syms, st.base = syms, ptrs, len(syms)
	compactions.Add(1)
}

// factIDs calls fn with each element f refers to but its own.
func factIDs(f *Fact, fn func(*ElementID)) {
	typeIDs(f.Type, fn)
	callIDs(f.Call, fn)
	idPtr(f.ReturnTarget, fn)
	idPtr(f.ReceiverTarget, fn)
	idPtr(f.ImplicitReceiver, fn)
	typeIDs(f.DelegateType, fn)
	for i := range f.DelegatedCalls {
		callIDs(&f.DelegatedCalls[i], fn)
	}
	for i := range f.Accessors {
		callIDs(&f.Accessors[i], fn)
	}
}

// symbolIDs calls fn with each element s refers to.
func symbolIDs(s *Symbol, fn func(*ElementID)) {
	typeIDs(&s.Type, fn)
	idPtr(s.Declaration, fn)
	for i := range s.Overrides {
		fn(&s.Overrides[i])
	}
}

func idPtr(id *ElementID, fn func(*ElementID)) {
	if id != nil {
		fn(id)
	}
}

func typeIDs(t *Type, fn func(*ElementID)) {
	if t == nil {
		return
	}
	idPtr(t.ParameterOwner, fn)
	for i := range t.Arguments {
		typeIDs(&t.Arguments[i], fn)
	}
}

func callIDs(c *Call, fn func(*ElementID)) {
	if c == nil {
		return
	}
	idPtr(c.Declaration, fn)
	typeIDs(c.DeclaredReceiver, fn)
	typeIDs(c.Receiver, fn)
	idPtr(c.ImplicitReceiver, fn)
	for i := range c.Parameters {
		typeIDs(&c.Parameters[i], fn)
	}
	typeIDs(c.ReturnType, fn)
	typeIDs(c.DeclaredReturnType, fn)
	for i := range c.Targets {
		callIDs(&c.Targets[i], fn)
	}
}
