package analysis

import (
	"cmp"
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
// Kept facts are rewritten for the new update: what they refer to in a
// changed file's last parse is mapped to its new parse, whose nodes
// outside block bodies correspond one to one (kasession.SignatureNodes),
// and their symbols are numbered again. A kept fact's symbol is the new
// update's if the new facts name it too: the same declaration in the
// sources, or the same symbol of the library; else it is added.
// A library's symbol that only kept facts name is described as the
// update that found it did, and an update's own facts may name it again
// under another number.

// incrementalUpdates counts the updates that check only some files, for
// tests.
var incrementalUpdates atomic.Int64

// state is what an incremental update starts from: the last update's
// snapshots, and each file's facts and diagnostics.
type state struct {
	res   *Result
	snaps []*psi.Snapshot
	facts [][]*Fact // by file, in node order
	diags [][]Diagnostic
	// syms are the symbols of res.symbols as the checker made them, or nil
	// for one an earlier update found.
	syms []any
}

// newState returns the state of res, an update of snaps whose every error
// is one file's, and syms its symbols; nil if two snapshots have the same
// path.
func newState(res *Result, snaps []*psi.Snapshot, syms []any) *state {
	st := &state{res: res, snaps: snaps, facts: make([][]*Fact, len(snaps)), diags: make([][]Diagnostic, len(snaps)),
		syms: make([]any, len(res.symbols))}
	copy(st.syms, syms)
	byPath := map[string]int{}
	byID := map[psi.SnapshotID]int{}
	for i, s := range snaps {
		if _, dup := byPath[s.Path()]; dup {
			return nil
		}
		byPath[s.Path()] = i
		byID[s.ID()] = i
	}
	for _, id := range res.order {
		i := byID[id.Snapshot]
		st.facts[i] = append(st.facts[i], res.facts[id])
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
	// The kept facts and diagnostics, their elements mapped first: one in
	// a changed file's body means the update must be checked whole.
	m := &remapper{nodes: nodes, ok: true}
	kept := make([][]Fact, len(snaps))
	for i, fs := range st.facts {
		if isChanged[i] {
			continue
		}
		kept[i] = make([]Fact, len(fs))
		for j, f := range fs {
			c := copyFact(*f)
			factIDs(&c, m.element)
			kept[i][j] = c
		}
	}
	if !m.ok {
		return nil, nil, false, nil
	}

	jobs := max(cfg.Jobs, 1)
	n := len(snaps)
	files := make([]*syntax.File, n)
	provs := make([]*lower.Provenance, n)
	errLists := make([][]*syntax.Error, n)
	parallel(n, jobs, func(i int) {
		files[i], provs[i], errLists[i] = lower.File(snaps[i])
	})
	if err := ctx.Err(); err != nil {
		return nil, nil, false, err
	}
	diags := make([][]Diagnostic, n)
	broken := make([]bool, n)
	for i := range snaps {
		if !isChanged[i] {
			diags[i] = st.diags[i]
			broken[i] = true // its facts are kept
			continue
		}
		for _, e := range errLists[i] {
			diags[i] = append(diags[i], diagnostic(snaps[i], e))
			broken[i] = true
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
		diags[i] = append(diags[i], diagnostic(snaps[i], e))
		broken[i] = true
	}

	res = &Result{facts: map[ElementID]*Fact{}}
	var syms []any
	if slices.Contains(broken, false) {
		b := newBuilder(res, snaps, files, provs, rec)
		b.cp = cp
		b.broken = broken
		b.build()
		syms = b.syms
	}
	next = &state{res: res, snaps: snaps, facts: make([][]*Fact, n), diags: diags, syms: make([]any, len(res.symbols))}
	copy(next.syms, syms)
	byPtr := map[any]uint32{}
	for i, p := range next.syms {
		byPtr[p] = uint32(i + 1)
	}
	type declKey struct {
		el         ElementID
		kind, name string
	}
	byDecl := map[declKey]uint32{}
	for _, s := range res.symbols {
		if s.Declaration != nil {
			byDecl[declKey{*s.Declaration, s.Kind, s.Name}] = s.ID
		}
	}
	// The kept facts' symbols, numbered again.
	ids := map[uint32]uint32{}
	symbol := func(old *uint32) {
		if *old == 0 {
			return
		}
		if id, ok := ids[*old]; ok {
			*old = id
			return
		}
		var id uint32
		s := copySymbol(st.res.symbols[*old-1])
		p := st.syms[*old-1]
		symbolIDs(&s, m.element)
		if p != nil {
			id = byPtr[p]
		}
		if id == 0 && s.Declaration != nil {
			id = byDecl[declKey{*s.Declaration, s.Kind, s.Name}]
		}
		if id == 0 {
			id = uint32(len(res.symbols) + 1)
			s.ID = id
			res.symbols = append(res.symbols, s)
			next.syms = append(next.syms, nil)
		}
		ids[*old] = id
		*old = id
	}
	for i := range snaps {
		if isChanged[i] {
			continue
		}
		for j := range kept[i] {
			f := &kept[i][j]
			symbol(&f.Symbol)
			for k := range f.LambdaParameters {
				symbol(&f.LambdaParameters[k])
			}
			res.facts[f.Element] = f
			res.order = append(res.order, f.Element)
		}
	}
	if !m.ok {
		// A kept symbol declared in a changed file's body: a local
		// whose fact was dropped. Can't be: be safe.
		return nil, nil, false, nil
	}
	fileOf := map[psi.SnapshotID]int{}
	for i, s := range snaps {
		fileOf[s.ID()] = i
	}
	slices.SortFunc(res.order, func(a, c ElementID) int {
		if r := cmp.Compare(fileOf[a.Snapshot], fileOf[c.Snapshot]); r != 0 {
			return r
		}
		return cmp.Compare(a.Node, c.Node)
	})
	for _, id := range res.order {
		i := fileOf[id.Snapshot]
		next.facts[i] = append(next.facts[i], res.facts[id])
	}
	for _, ds := range diags {
		res.diags = append(res.diags, ds...)
	}
	incrementalUpdates.Add(1)
	return res, next, true, nil
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
