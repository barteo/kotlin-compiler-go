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
	// lookups are the names each file looked up when it was last checked,
	// sorted, and decls what each declares.
	lookups [][]lookup
	decls   []declInfo
}

// newState returns the state of res, an update of snaps whose every error
// is one file's, and syms its symbols; nil if two snapshots have the same
// path.
func newState(res *Result, snaps []*psi.Snapshot, syms []any) *state {
	if res.files == nil {
		res.setFiles(snaps, make([][]*Fact, len(snaps)))
	}
	st := &state{res: res, snaps: snaps, diags: make([][]Diagnostic, len(snaps)), refs: make([][]psi.SnapshotID, len(snaps)),
		syms: make([]any, len(res.symbols)), lookups: make([][]lookup, len(snaps)), decls: make([]declInfo, len(snaps))}
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

// changes is how an update's snapshots differ from a state's.
type changes struct {
	old     []int  // each snapshot's index in the state's, or -1 if added
	changed []bool // a snapshot that isn't the state's
	nodes   map[psi.SnapshotID]*nodeMap
	decls   []declInfo
	// names are the names declared differently, and supers the names of
	// the supertypes of the classes among them.
	names  map[lookup]bool
	supers map[string]bool
}

// changes returns how snaps differ from st's snapshots; false if two
// have the same path. A changed file's nodes are mapped whole if only
// bodies changed, and else as far as AlignSignatureNodes maps them; a
// removed file's map nothing.
func (st *state) changes(snaps []*psi.Snapshot) (*changes, bool) {
	c := &changes{old: make([]int, len(snaps)), changed: make([]bool, len(snaps)), nodes: map[psi.SnapshotID]*nodeMap{},
		decls: make([]declInfo, len(snaps)), names: map[lookup]bool{}, supers: map[string]bool{}}
	byPath := make(map[string]int, len(st.snaps))
	for j, s := range st.snaps {
		byPath[s.Path()] = j
	}
	seen := make([]bool, len(st.snaps))
	paths := make(map[string]bool, len(snaps))
	for i, s := range snaps {
		if paths[s.Path()] {
			return nil, false
		}
		paths[s.Path()] = true
		j, ok := byPath[s.Path()]
		if !ok {
			c.old[i], c.changed[i] = -1, true
			c.decls[i] = declared(s)
			touched(declInfo{}, c.decls[i], c.names, c.supers)
			continue
		}
		c.old[i], seen[j] = j, true
		old := st.snaps[j]
		c.decls[i] = st.decls[j]
		if s == old {
			continue
		}
		c.changed[i] = true
		m := &nodeMap{snap: s.ID()}
		if kasession.SignatureHash(s) == kasession.SignatureHash(old) {
			m.old, m.cur = kasession.SignatureNodes(old), kasession.SignatureNodes(s)
			if len(m.old) == len(m.cur) {
				c.nodes[old.ID()] = m
				continue
			}
		}
		// What it declares changed.
		m.old, m.cur = kasession.AlignSignatureNodes(old, s)
		c.nodes[old.ID()] = m
		c.decls[i] = declared(s)
		touched(st.decls[j], c.decls[i], c.names, c.supers)
	}
	for j, s := range st.snaps {
		if !seen[j] {
			c.nodes[s.ID()] = &nodeMap{}
			touched(st.decls[j], declInfo{}, c.names, c.supers)
		}
	}
	return c, true
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
// changed files and those a declaration edit affects, and its state; ok
// is false if the update must be checked whole.
func incremental(ctx context.Context, cfg Config, snaps []*psi.Snapshot, st *state, open openLibrary) (res *Result, next *state, ok bool, err error) {
	c, ok := st.changes(snaps)
	if !ok {
		return nil, nil, false, nil
	}
	n := len(snaps)
	if !slices.Contains(c.changed, true) && n == len(st.snaps) {
		return st.res, st, true, nil
	}
	nodes := c.nodes
	// dirty are the files checked: the changed ones, those that looked up
	// or declare a name declared differently, and those whose kept facts
	// would refer to what a changed file no longer declares.
	dirty := slices.Clone(c.changed)
	if len(c.names) > 0 {
		for i := range snaps {
			if !dirty[i] && affected(st.lookups[c.old[i]], c.decls[i], c.names, c.supers) {
				dirty[i] = true
			}
		}
	}
	// The kept symbols and facts, their elements mapped first: a kept fact
	// that names a symbol declared in what a changed file no longer has,
	// or refers to that, is its file's to check. What doesn't refer to a
	// changed file is shared with the last update's Result, which doesn't
	// change.
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
			dead[i] = true // what a changed file no longer declares: no kept fact may name it
			continue
		}
		syms[i] = s
	}
	isDead := func(id uint32) bool { return id != 0 && dead[id-1] }
	kept := make([][]*Fact, n)
	refs := make([][]psi.SnapshotID, n)
	for i := range snaps {
		if dirty[i] {
			continue
		}
		j := c.old[i]
		fs := st.res.files[j]
		kept[i], refs[i] = fs, st.refs[j]
		if slices.ContainsFunc(fs, func(f *Fact) bool {
			return isDead(f.Symbol) || slices.ContainsFunc(f.LambdaParameters, isDead)
		}) {
			dirty[i] = true
			continue
		}
		if !slices.ContainsFunc(refs[i], func(id psi.SnapshotID) bool { return nodes[id] != nil }) {
			continue // shared
		}
		m.ok = true
		mapped := make([]*Fact, len(fs))
		for k, f := range fs {
			if touches(func(fn func(*ElementID)) { factIDs(f, fn) }) {
				c := copyFact(*f)
				factIDs(&c, m.element)
				f = &c
			}
			mapped[k] = f
		}
		if !m.ok {
			dirty[i] = true // it refers to what a changed file no longer declares
			continue
		}
		kept[i] = mapped
		refs[i] = slices.Clone(refs[i])
		for k, id := range refs[i] {
			if nm := nodes[id]; nm != nil {
				refs[i][k] = nm.snap
			}
		}
	}
	ndirty := 0
	for _, d := range dirty {
		if d {
			ndirty++
		}
	}
	if len(c.names) > 0 && ndirty*2 > n {
		// A declaration edit that affects most files: as cheap to check
		// whole, which also drops the symbols only the others declared.
		return nil, nil, false, nil
	}

	jobs := max(cfg.Jobs, 1)
	files := make([]*syntax.File, n)
	provs := make([]*lower.Provenance, n)
	errLists := make([][]*syntax.Error, n)
	parallel(n, jobs, func(i int) {
		if dirty[i] {
			files[i], provs[i], errLists[i] = lower.Recover(snaps[i])
		} else {
			// Only declared: its facts and diagnostics are kept.
			files[i], provs[i], _ = lower.Signatures(snaps[i])
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, nil, false, err
	}
	if duplicateClasses(files) != nil {
		return nil, nil, false, nil
	}
	diags := make([][]Diagnostic, n)
	broken := make([]bool, n)
	drops := make([][]span, n)
	for i := range snaps {
		if !dirty[i] {
			diags[i] = st.diags[c.old[i]]
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
	for i := range snaps {
		if dirty[i] {
			only[files[i]] = true
		}
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
		if !found || !dirty[i] {
			return nil, nil, false, nil // not a checked file's
		}
		d := diagnostic(snaps[i], e)
		diags[i] = append(diags[i], d)
		drop(snaps[i], d, &drops[i], &broken[i])
	}

	// The checked files' facts, their symbols numbered into the kept
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
	next = &state{res: res, snaps: snaps, diags: diags, refs: refs, syms: make([]any, len(syms)), base: st.base,
		lookups: make([][]lookup, n), decls: c.decls}
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
		if !dirty[i] {
			all[i] = kept[i]
			next.lookups[i] = st.lookups[c.old[i]]
		} else {
			next.lookups[i] = rec.fileLookups(files[i])
			if built.files != nil {
				all[i] = built.files[i]
				for _, f := range all[i] {
					renumber(&f.Symbol)
					for k := range f.LambdaParameters {
						renumber(&f.LambdaParameters[k])
					}
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
