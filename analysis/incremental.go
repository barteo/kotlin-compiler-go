package analysis

import (
	"context"
	"hash/fnv"
	"slices"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Incremental updates. An editor changes one file at a time, and mostly
// a body: a Session's update whose changed files declare what they did
// (signatureHash) checks only those files (check.Options.Only), builds
// their facts, and keeps the others' facts and diagnostics from the last
// update. What a file's facts and errors say depends on its own text and
// on what the other files declare, not on their bodies, so the others'
// are still right. An update that changes what files declare also checks
// the files that looked up a name they declared before or after
// (declared.go). Every other update is checked whole: one whose options
// changed, or most of whose files a declaration edit affects, or where
// an error isn't one file's or lowering may have left out a declaration.
//
// Kept facts and symbols keep their numbers, and are shared with the last
// update's Result unless they refer to a changed file: what they refer to
// in its last parse is mapped to its new parse, whose nodes outside block
// bodies correspond one to one (signatureNodes). The changed files'
// facts name the kept symbols where they name the same declaration in the
// sources, or the same symbol of the library as the last update's own
// facts did; other symbols are added. A library's symbol that only kept
// facts name is described as the update that found it did, and an
// update's own facts may name it again under another number. So a
// Result's symbols may include some no fact names, until there are too
// many of them (state.compact).
//
// The module-wide filters of the diagnostics apply to each update's
// (finish): each file's are kept as the checker reported them.

// state is what an incremental update starts from: the last update's
// snapshots, and each file's facts and diagnostics.
type state struct {
	key   optionsKey
	res   *Result // its files are each file's facts
	snaps []*psi.Snapshot
	// diags are each file's diagnostics, before finish; broken says
	// which files were lowered recovering from syntax errors; extra are
	// the diagnostics of no snapshot, the Java sources'.
	diags  [][]fileDiag
	broken []bool
	extra  []Diagnostic
	// lost says which files lowering may have left a declaration of out:
	// the other files were checked without it, so any change to such a
	// file changes all it declares.
	lost []bool
	// lostDecls says which files recovery left out of what the rest of
	// the module sees (lowered.lostDecls).
	lostDecls []bool
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
	// sorted; decls what each declares, and hashes their signatureHash.
	lookups [][]lookup
	decls   []declInfo
	hashes  []uint64
	// declared are the files only declared (Options.Declared), or nil.
	declared []bool
}

// optionsKey is what of an update's Options its state depends on: an
// update with other options is checked whole.
type optionsKey struct {
	warnings, extra bool
	java            uint64 // a hash of the Java sources
}

func keyOf(opts Options) optionsKey {
	k := optionsKey{warnings: opts.Warnings, extra: opts.Warnings && opts.ExtraWarnings}
	if len(opts.Java) > 0 {
		h := fnv.New64a()
		for _, j := range opts.Java {
			h.Write([]byte(j.Path))
			h.Write([]byte{0})
			h.Write([]byte(j.Text))
			h.Write([]byte{0})
		}
		k.java = h.Sum64() | 1
	}
	return k
}

// newState returns the state of res, an update of snaps whose every error
// is one file's, and syms its symbols; nil if two snapshots have the same
// path. The caller sets diags, broken, extra and declared.
func newState(res *Result, snaps []*psi.Snapshot, syms []any, opts Options) *state {
	if res.files == nil {
		res.setFiles(snaps, make([]fileFacts, len(snaps)))
	}
	if len(res.files) != len(snaps) {
		return nil // a file was left out
	}
	n := len(snaps)
	st := &state{key: keyOf(opts), res: res, snaps: snaps, refs: make([][]psi.SnapshotID, n),
		syms: make([]any, len(res.symbols)), lookups: make([][]lookup, n), decls: make([]declInfo, n), hashes: make([]uint64, n)}
	copy(st.syms, syms)
	st.base = len(res.symbols)
	seen := map[string]bool{}
	for i, s := range snaps {
		if seen[s.Path()] {
			return nil
		}
		seen[s.Path()] = true
		st.refs[i] = fileRefs(s.ID(), &res.files[i])
		st.hashes[i] = signatureHash(s)
	}
	return st
}

// fileRefs returns the snapshots but own that the facts of ff refer to.
func fileRefs(own psi.SnapshotID, ff *fileFacts) []psi.SnapshotID {
	var refs []psi.SnapshotID
	var f Fact
	var v view
	for i := range ff.facts {
		ff.load(i, &f, &v)
		factIDs(&f, func(id *ElementID) {
			if id.Snapshot != own && !slices.Contains(refs, id.Snapshot) {
				refs = append(refs, id.Snapshot)
			}
		})
	}
	return refs
}

// anySymbol reports whether a fact of ff names a symbol pred reports.
func (ff *fileFacts) anySymbol(pred func(uint32) bool) bool {
	for i := range ff.facts {
		if pred(ff.facts[i].symbol) {
			return true
		}
	}
	for i := range ff.extras {
		if slices.ContainsFunc(ff.extras[i].lambdaParameters, pred) {
			return true
		}
	}
	return false
}

// own returns a copy of ff that its caller may change.
func (ff *fileFacts) own() fileFacts {
	return fileFacts{snap: ff.snap, facts: slices.Clone(ff.facts), extras: slices.Clone(ff.extras)}
}

// renumber changes the symbols the facts of ff name by fn, in place: ff
// must be its own, but the lambdas' parameters, which may be shared, are
// set anew.
func (ff *fileFacts) renumber(fn func(uint32) uint32) {
	for i := range ff.facts {
		ff.facts[i].symbol = fn(ff.facts[i].symbol)
	}
	for i := range ff.extras {
		x := &ff.extras[i]
		if x.lambdaParameters != nil {
			ps := make([]uint32, len(x.lambdaParameters))
			for k, p := range x.lambdaParameters {
				ps[k] = fn(p)
			}
			x.lambdaParameters = ps
		}
	}
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
	hashes  []uint64 // each snapshot's signatureHash
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
		decls: make([]declInfo, len(snaps)), hashes: make([]uint64, len(snaps)), names: map[lookup]bool{}, supers: map[string]bool{}}
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
			c.hashes[i] = signatureHash(s)
			c.decls[i] = declared(s)
			touched(declInfo{}, c.decls[i], c.names, c.supers)
			continue
		}
		c.old[i], seen[j] = j, true
		old := st.snaps[j]
		c.decls[i], c.hashes[i] = st.decls[j], st.hashes[j]
		if s == old {
			continue
		}
		c.changed[i] = true
		c.hashes[i] = signatureHash(s)
		m := &nodeMap{snap: s.ID()}
		if st.hashes[j] == c.hashes[i] {
			m.old, m.cur = signatureNodes(old), signatureNodes(s)
			if len(m.old) == len(m.cur) {
				c.nodes[old.ID()] = m
				continue
			}
		}
		// What it declares changed.
		m.old, m.cur = alignSignatureNodes(old, s)
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
// is false if the update must be checked whole. A file only declared
// (Options.Declared) is never checked: one whose declarations changed
// affects the others as a changed file does.
func (l *Library) incremental(ctx context.Context, snaps []*psi.Snapshot, opts Options, st *state) (res *Result, next *state, ok bool, err error) {
	if keyOf(opts) != st.key {
		return nil, nil, false, whole("options")
	}
	c, ok := st.changes(snaps)
	if !ok {
		return nil, nil, false, whole("duplicate paths")
	}
	n := len(snaps)
	declOnly := declaredOf(snaps, opts.Declared)
	isDeclared := func(i int) bool { return declOnly != nil && declOnly[i] }
	own := n // the files checked whole
	for i := range snaps {
		if isDeclared(i) {
			own--
		}
		if j := c.old[i]; j >= 0 && isDeclared(i) != (st.declared != nil && st.declared[j]) {
			return nil, nil, false, whole("declared flipped") // a file declared or checked that wasn't
		}
	}
	if !slices.Contains(c.changed, true) && n == len(st.snaps) {
		return st.res, st, true, nil
	}
	// The changed files are lowered first: one that lowering leaves, or
	// left, a declaration of out may declare anything differently.
	jobs := max(l.cfg.Jobs, 1)
	low := make([]lowered, n)
	parallel(n, jobs, func(i int) {
		if c.changed[i] {
			low[i] = lowerSnapshot(snaps[i])
		}
	})
	for i := range snaps {
		if !c.changed[i] {
			continue
		}
		if low[i].file == nil {
			return nil, nil, false, whole("a file left out")
		}
		if j := c.old[i]; low[i].lost || j >= 0 && st.lost[j] {
			if j >= 0 {
				touched(st.decls[j], declInfo{}, c.names, c.supers)
			}
			touched(declInfo{}, c.decls[i], c.names, c.supers)
		}
	}
	if len(c.names) > 0 && len(opts.Java) > 0 {
		// The Java sources see the Kotlin declarations: checked whole.
		return nil, nil, false, whole("java")
	}
	nodes := c.nodes
	// dirty are the files checked: the changed ones, those that looked up
	// or declare a name declared differently, and those whose kept facts
	// would refer to what a changed file no longer declares.
	dirty := slices.Clone(c.changed)
	for i := range dirty {
		dirty[i] = dirty[i] && !isDeclared(i)
	}
	if len(c.names) > 0 {
		for i := range snaps {
			if !dirty[i] && !isDeclared(i) && affected(st.lookups[c.old[i]], c.decls[i], c.names, c.supers) {
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
	kept := make([]fileFacts, n)
	refs := make([][]psi.SnapshotID, n)
	var remapped *store // stores the facts mapped, which share no values with the last Result's
	for i := range snaps {
		if dirty[i] || isDeclared(i) {
			continue
		}
		j := c.old[i]
		ff := &st.res.files[j]
		kept[i], refs[i] = *ff, st.refs[j]
		if ff.anySymbol(isDead) {
			dirty[i] = true
			continue
		}
		if !slices.ContainsFunc(refs[i], func(id psi.SnapshotID) bool { return nodes[id] != nil }) {
			continue // shared
		}
		m.ok = true
		var mapped *fileFacts
		var f Fact
		var v view
		for k := range ff.facts {
			ff.load(k, &f, &v)
			if !touches(func(fn func(*ElementID)) { factIDs(&f, fn) }) {
				continue
			}
			c := copyFact(f)
			factIDs(&c, m.element)
			if mapped == nil {
				o := ff.own()
				mapped = &o
			}
			if remapped == nil {
				remapped = newStore(nil)
			}
			mapped.set(k, &c, remapped)
		}
		if !m.ok {
			dirty[i] = true // it refers to what a changed file no longer declares
			continue
		}
		if mapped != nil {
			kept[i] = *mapped
		}
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
	if len(c.names) > 0 && ndirty*2 > own {
		// A declaration edit that affects most files: as cheap to check
		// whole, which also drops the symbols only the others declared.
		return nil, nil, false, whole("most files")
	}

	// The files checked are lowered whole, the others without the bodies
	// the check doesn't read.
	parallel(n, jobs, func(i int) {
		switch {
		case c.changed[i]:
		case dirty[i]:
			low[i] = lowerSnapshot(snaps[i])
		default:
			low[i] = lowerSignatures(snaps[i])
			low[i].lost = st.lost[c.old[i]]
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, nil, false, err
	}
	files := make([]*syntax.File, n)
	provs := make([]*lower.Provenance, n)
	diags := make([][]fileDiag, n)
	broken := make([]bool, n)
	lostDecls := make([]bool, n)
	anyLostDecls := false
	for i := range snaps {
		lw := low[i]
		if lw.file == nil {
			return nil, nil, false, whole("a file left out")
		}
		files[i], provs[i] = lw.file, lw.prov
		switch {
		case isDeclared(i):
		case !dirty[i]:
			diags[i], broken[i], lostDecls[i] = st.diags[c.old[i]], st.broken[c.old[i]], st.lostDecls[c.old[i]]
		default:
			broken[i], lostDecls[i] = lw.broken, lw.lostDecls
			for _, e := range lw.errs {
				diags[i] = append(diags[i], fileDiag{diagnostic(snaps[i].Path(), e), diagLower})
			}
		}
		anyLostDecls = anyLostDecls || lostDecls[i]
	}
	if duplicateClasses(files) != nil {
		return nil, nil, false, whole("duplicate classes")
	}
	cp, lib, extra := l.withJava(opts.Java, files)
	only := map[*syntax.File]bool{}
	for i := range snaps {
		if dirty[i] {
			only[files[i]] = true
		}
	}
	rec := newRecorder()
	mode, _ := check.ParseAPIMode(l.cfg.ExplicitAPIMode)
	copts := check.Options{
		ExplicitAPI: mode, Warnings: opts.Warnings, ExtraWarnings: opts.Warnings && opts.ExtraWarnings, Raw: true, JVMTarget: l.major, Only: only,
	}
	errs, err := check.CheckOptions(ctx, files, lib, jobs, lookupRecorder{rec}, copts)
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
			return nil, nil, false, whole("error of an unchecked file") // not a checked file's
		}
		if !e.IsWarning() && low[i].lines[e.Pos.Line] || lostWarning(low[i].file, e) {
			continue
		}
		diags[i] = append(diags[i], checkerDiag(snaps[i].Path(), e))
	}

	// The checked files' facts, their symbols numbered into the kept
	// table: the same symbol of the library as the last update's facts
	// named, or the same declaration in the sources; else added.
	built := &Result{}
	var builtSyms []any
	// Also for a changed declared file, whose classes the symbols have
	// (builder.build).
	build := false
	for i := range snaps {
		build = build || dirty[i] || c.changed[i] && isDeclared(i)
	}
	if build {
		b := newBuilder(built, snaps, files, provs, rec)
		b.cp = cp
		b.broken = make([]bool, n)
		for i := range dirty {
			b.broken[i] = !dirty[i]
		}
		b.build()
		builtSyms = b.syms
	}
	res = &Result{symbols: syms}
	next = &state{key: st.key, res: res, snaps: snaps, diags: diags, broken: broken, extra: extra, lost: make([]bool, n), lostDecls: lostDecls, refs: refs, syms: make([]any, len(syms)), base: st.base,
		lookups: make([][]lookup, n), decls: c.decls, hashes: c.hashes, declared: declOnly}
	// next.syms keeps the pointers of this update's symbols only: an
	// earlier check's would keep its sources' symbols alive.
	byPtr := map[any]uint32{}
	for i, p := range st.syms {
		if p != nil && !dead[i] {
			byPtr[p] = uint32(i + 1)
		}
	}
	byDecl := map[declKey]uint32{}
	for i, s := range syms {
		if s.Declaration != nil && !dead[i] {
			byDecl[declKey{*s.Declaration, s.Kind, s.Name}] = s.ID
		}
	}
	for i := range low {
		next.lost[i] = low[i].lost
	}
	keptSnap := map[psi.SnapshotID]bool{}
	for i, snap := range snaps {
		if !dirty[i] && !isDeclared(i) {
			keptSnap[snap.ID()] = true
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
		switch {
		case id == 0:
			id = uint32(len(res.symbols) + 1)
			res.symbols = append(res.symbols, Symbol{})
			next.syms = append(next.syms, nil)
		case s.Declaration != nil && keptSnap[s.Declaration.Snapshot]:
			// Declared in a file this update didn't check, which described
			// it as its own check found it: this check may not have
			// inferred what it didn't need, as an omitted type.
			ids[k] = id
			continue
		}
		s.ID = id
		res.symbols[id-1] = s
		next.syms[id-1] = p
		ids[k] = id
	}
	renumber := func(id uint32) uint32 {
		if id != 0 {
			return ids[id-1]
		}
		return 0
	}
	all := make([]fileFacts, n)
	for i := range snaps {
		switch {
		case isDeclared(i):
		case !dirty[i]:
			all[i] = kept[i]
			next.lookups[i] = st.lookups[c.old[i]]
		default:
			next.lookups[i] = rec.fileLookups(files[i])
			if built.files != nil {
				all[i] = built.files[i] // this update's own
				all[i].renumber(renumber)
			}
			refs[i] = fileRefs(snaps[i].ID(), &all[i])
		}
	}
	res.setFiles(snaps, all)
	res.diags = finish(extra, diags, anyLostDecls)
	if len(res.symbols) > 2*next.base+compactSlack {
		next.compact()
	}
	return res, next, true, nil
}

// traceWhole, if it isn't nil, gets why an update is checked whole, for
// tests.
var traceWhole func(reason string)

// whole reports why an update is checked whole to traceWhole.
func whole(reason string) error {
	if traceWhole != nil {
		traceWhole(reason)
	}
	return nil
}

// compactSlack is how many symbols beyond twice the last numbering's an
// update may have before it compacts them.
var compactSlack = 256

type declKey struct {
	el         ElementID
	kind, name string
}

// compact numbers the symbols that facts name and the classes the sources
// declare again, dropping the others: those only the facts of changed
// files' earlier versions named.
func (st *state) compact() {
	res := st.res
	ids := make([]uint32, len(res.symbols))
	mark := func(id uint32) {
		if id != 0 {
			ids[id-1] = 1
		}
	}
	current := make(map[psi.SnapshotID]bool, len(st.snaps))
	for _, s := range st.snaps {
		current[s.ID()] = true
	}
	for i, s := range res.symbols {
		if s.Kind == "class" && s.Declaration != nil && current[s.Declaration.Snapshot] {
			ids[i] = 1
		}
	}
	for i := range res.files {
		res.files[i].anySymbol(func(id uint32) bool {
			mark(id)
			return false
		})
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
	for i := range res.files {
		ff := &res.files[i]
		if !ff.anySymbol(moved) {
			continue
		}
		// A copy: the files may be an earlier Result's.
		*ff = ff.own()
		ff.renumber(func(id uint32) uint32 {
			if id != 0 {
				return ids[id-1]
			}
			return 0
		})
	}
	res.symbols, st.syms, st.base = syms, ptrs, len(syms)
}

// factIDs calls fn with each element f refers to but its own.
func factIDs(f *Fact, fn func(*ElementID)) {
	typeIDs(f.Type, fn)
	callIDs(f.Call, fn)
	idPtr(f.ReturnTarget, fn)
	idPtr(f.ReceiverTarget, fn)
	idPtr(f.ImplicitReceiver, fn)
	typeIDs(f.DelegateType, fn)
	typeIDs(f.LambdaReceiver, fn)
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
