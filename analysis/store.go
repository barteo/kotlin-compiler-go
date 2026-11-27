package analysis

import (
	"hash/maphash"
	"slices"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// A Result stores its facts compactly. Each file's are records of the
// fields most facts set, by value, in node order, with the fields few
// facts set in a side table. Types are interned; a call is its callee's
// signature, which the calls of the callee share, and what is the call's
// own, interned too, so that a call and its callee's reference share
// one. Stored facts never change: an update that changes one stores it
// anew, with values of its own.

// storedFact is a stored Fact.
type storedFact struct {
	node   psi.NodeID
	role   uint8  // of roles
	bits   uint8  // the bools, by the bit constants below
	extra  uint32 // 1 + the index of its factExtra; 0 for none
	symbol uint32
	typ    *Type
	call   *callRec
}

// The bits of storedFact.bits.
const (
	fClassifier = 1 << iota
	fFlowNonNull
	fDefinitelyNonNull
	fImpossibleCast
	fUsedAsExpression
	fUnreachable
	fSuspendOperation
	fImplicitParameter
)

// roles are Fact.Role's values, by storedFact.role.
var roles = []string{"", "declaration", "reference", "expression", "type", "statement"}

func roleIndex(role string) uint8 {
	if i := slices.Index(roles, role); i >= 0 {
		return uint8(i)
	}
	panic("analysis: fact role " + role + " isn't stored")
}

// factExtra is the fields of a stored Fact that few facts set.
type factExtra struct {
	builtin, importableName, pkg string
	when                         *When
	nonNullLeftSubtypeRight      *bool
	deprecated                   *string
	lambdaParameters             []uint32
	lambdaReceiver, delegateType *Type
	returnTarget, receiverTarget *ElementID
	implicitReceiver             *ElementID
	delegatedCalls, accessors    []*callRec
}

// callRec is a stored Call: its callee's signature, a Call whose fields
// that are a call's own are zero, and those fields.
type callRec struct {
	sig              *Call
	receiver         *Type
	returnType       *Type
	implicitReceiver *ElementID
	argMap           []int
	valueInvocation  bool
}

// fileFacts are the stored facts of a file, its snapshot's.
type fileFacts struct {
	snap   psi.SnapshotID
	facts  []storedFact
	extras []factExtra
}

// find returns the index of node's fact.
func (ff *fileFacts) find(node psi.NodeID) (int, bool) {
	return slices.BinarySearchFunc(ff.facts, node, func(f storedFact, n psi.NodeID) int {
		switch {
		case f.node < n:
			return -1
		case f.node > n:
			return 1
		}
		return 0
	})
}

// view holds what a fact viewed in place points to that isn't stored as
// a public value: its calls.
type view struct {
	call                      Call
	accessors, delegatedCalls []Call
}

// load sets *f to the ith fact, sharing the stored values: neither f nor
// what it points to may be changed, and it is only valid until v is used
// again.
func (ff *fileFacts) load(i int, f *Fact, v *view) {
	x := ff.loadPlain(i, f)
	if c := ff.facts[i].call; c != nil {
		c.load(&v.call)
		f.Call = &v.call
	}
	if x != nil {
		f.Accessors = loadCalls(x.accessors, &v.accessors)
		f.DelegatedCalls = loadCalls(x.delegatedCalls, &v.delegatedCalls)
	}
}

// loadPlain sets *f to the ith fact, as load does, but its calls, and
// returns its factExtra, if any.
func (ff *fileFacts) loadPlain(i int, f *Fact) *factExtra {
	s := &ff.facts[i]
	*f = Fact{
		Element: ElementID{ff.snap, s.node},
		Role:    roles[s.role],
		Type:    s.typ,
		Symbol:  s.symbol,

		Classifier:        s.bits&fClassifier != 0,
		FlowNonNull:       s.bits&fFlowNonNull != 0,
		DefinitelyNonNull: s.bits&fDefinitelyNonNull != 0,
		ImpossibleCast:    s.bits&fImpossibleCast != 0,
		UsedAsExpression:  s.bits&fUsedAsExpression != 0,
		Unreachable:       s.bits&fUnreachable != 0,
		SuspendOperation:  s.bits&fSuspendOperation != 0,
		ImplicitParameter: s.bits&fImplicitParameter != 0,
	}
	if s.extra == 0 {
		return nil
	}
	x := &ff.extras[s.extra-1]
	f.Builtin, f.ImportableName, f.Package = x.builtin, x.importableName, x.pkg
	f.When, f.NonNullLeftSubtypeRight, f.Deprecated = x.when, x.nonNullLeftSubtypeRight, x.deprecated
	f.LambdaParameters, f.LambdaReceiver, f.DelegateType = x.lambdaParameters, x.lambdaReceiver, x.delegateType
	f.ReturnTarget, f.ReceiverTarget, f.ImplicitReceiver = x.returnTarget, x.receiverTarget, x.implicitReceiver
	return x
}

// fact returns the ith fact, a copy the caller may change.
func (ff *fileFacts) fact(i int) Fact {
	var f Fact
	x := ff.loadPlain(i, &f)
	f = copyFact(f)
	f.Call = ff.facts[i].call.copy()
	if x != nil {
		f.Accessors = copyRecs(x.accessors)
		f.DelegatedCalls = copyRecs(x.delegatedCalls)
	}
	return f
}

// copy returns a copy of the call r stores; nil for nil.
func (r *callRec) copy() *Call {
	if r == nil {
		return nil
	}
	var c Call
	r.load(&c)
	v := copyCallValue(c)
	return &v
}

// copyRecs returns copies of the calls rs store.
func copyRecs(rs []*callRec) []Call {
	if rs == nil {
		return nil
	}
	out := make([]Call, len(rs))
	for i, r := range rs {
		var c Call
		r.load(&c)
		out[i] = copyCallValue(c)
	}
	return out
}

// noCalls is the empty, but not nil, calls.
var noCalls = []Call{}

// loadCalls returns the calls of rs in *buf, reusing its memory.
func loadCalls(rs []*callRec, buf *[]Call) []Call {
	switch {
	case rs == nil:
		return nil
	case len(rs) == 0:
		return noCalls
	}
	*buf = slices.Grow((*buf)[:0], len(rs))[:len(rs)]
	for i, r := range rs {
		r.load(&(*buf)[i])
	}
	return *buf
}

// load sets *c to the call r stores, sharing its values.
func (r *callRec) load(c *Call) {
	*c = *r.sig
	c.Receiver, c.ReturnType, c.ImplicitReceiver = r.receiver, r.returnType, r.implicitReceiver
	c.ArgumentMapping, c.ValueInvocation = r.argMap, r.valueInvocation
}

// A store interns the types and calls of the facts it stores.
type store struct {
	types *typeTable
	seed  maphash.Seed
	sigs  map[uint64][]*Call
	calls map[uint64][]*callRec
}

// newStore returns a store that interns types in types, or a table of
// its own if it is nil.
func newStore(types *typeTable) *store {
	if types == nil {
		types = newTypeTable()
	}
	return &store{types: types, seed: maphash.MakeSeed(), sigs: map[uint64][]*Call{}, calls: map[uint64][]*callRec{}}
}

// freezeFile stores facts, the facts of snapshot snap in node order.
func (st *store) freezeFile(snap psi.SnapshotID, facts []*Fact) fileFacts {
	ff := fileFacts{snap: snap, facts: make([]storedFact, len(facts))}
	for i, f := range facts {
		ff.set(i, f, st)
	}
	return ff
}

// set stores f as the ith fact, in place: ff must be its own.
func (ff *fileFacts) set(i int, f *Fact, st *store) {
	s := storedFact{node: f.Element.Node, role: roleIndex(f.Role), symbol: f.Symbol, typ: st.types.canon(f.Type), call: st.call(f.Call)}
	for bit, b := range []bool{f.Classifier, f.FlowNonNull, f.DefinitelyNonNull, f.ImpossibleCast, f.UsedAsExpression, f.Unreachable, f.SuspendOperation, f.ImplicitParameter} {
		if b {
			s.bits |= 1 << bit
		}
	}
	x := factExtra{
		builtin: f.Builtin, importableName: f.ImportableName, pkg: f.Package,
		when: f.When, nonNullLeftSubtypeRight: f.NonNullLeftSubtypeRight, deprecated: f.Deprecated,
		lambdaParameters: slices.Clone(f.LambdaParameters),
		lambdaReceiver:   st.types.canon(f.LambdaReceiver), delegateType: st.types.canon(f.DelegateType),
		returnTarget: f.ReturnTarget, receiverTarget: f.ReceiverTarget, implicitReceiver: f.ImplicitReceiver,
		delegatedCalls: st.callList(f.DelegatedCalls), accessors: st.callList(f.Accessors),
	}
	switch old := ff.facts[i].extra; {
	case old != 0:
		ff.extras[old-1] = x
		s.extra = old
	case !x.isZero():
		ff.extras = append(ff.extras, x)
		s.extra = uint32(len(ff.extras))
	}
	ff.facts[i] = s
}

// isZero reports whether x stores nothing.
func (x *factExtra) isZero() bool {
	return x.builtin == "" && x.importableName == "" && x.pkg == "" && x.when == nil && x.nonNullLeftSubtypeRight == nil &&
		x.deprecated == nil && x.lambdaParameters == nil && x.lambdaReceiver == nil && x.delegateType == nil &&
		x.returnTarget == nil && x.receiverTarget == nil && x.implicitReceiver == nil && x.delegatedCalls == nil && x.accessors == nil
}

func (st *store) callList(cs []Call) []*callRec {
	if cs == nil {
		return nil
	}
	out := make([]*callRec, len(cs))
	for i := range cs {
		out[i] = st.call(&cs[i])
	}
	return out
}

// call returns the interned record of c; nil for nil.
func (st *store) call(c *Call) *callRec {
	if c == nil {
		return nil
	}
	r := callRec{sig: st.sig(c), receiver: st.types.canon(c.Receiver), returnType: st.types.canon(c.ReturnType),
		implicitReceiver: c.ImplicitReceiver, argMap: c.ArgumentMapping, valueInvocation: c.ValueInvocation}
	var h maphash.Hash
	h.SetSeed(st.seed)
	writePtr(&h, r.sig)
	writePtr(&h, r.receiver)
	writePtr(&h, r.returnType)
	if r.implicitReceiver != nil {
		writeID(&h, *r.implicitReceiver)
	}
	for _, a := range r.argMap {
		writeInt(&h, uint64(a))
	}
	k := h.Sum64()
	for _, o := range st.calls[k] {
		if o.sig == r.sig && o.receiver == r.receiver && o.returnType == r.returnType && o.valueInvocation == r.valueInvocation &&
			sameID(o.implicitReceiver, r.implicitReceiver) && sameSlice(o.argMap, r.argMap) {
			return o
		}
	}
	r.implicitReceiver = copyID(r.implicitReceiver)
	r.argMap = slices.Clone(r.argMap)
	p := &r
	st.calls[k] = append(st.calls[k], p)
	return p
}

// sig returns the interned signature of c: c, its types interned, with
// the fields that are a call's own zero.
func (st *store) sig(c *Call) *Call {
	s := *c
	s.Receiver, s.ReturnType, s.ImplicitReceiver, s.ArgumentMapping, s.ValueInvocation = nil, nil, nil, nil, false
	s = st.canonCall(&s)
	var h maphash.Hash
	h.SetSeed(st.seed)
	hashCall(&h, &s)
	k := h.Sum64()
	for _, o := range st.sigs[k] {
		if sameCall(o, &s) {
			return o
		}
	}
	p := new(Call)
	*p = copyCallShape(s)
	st.sigs[k] = append(st.sigs[k], p)
	return p
}

// canonCall returns c with its types, its targets' too, interned.
func (st *store) canonCall(c *Call) Call {
	s := *c
	s.DeclaredReceiver = st.types.canon(s.DeclaredReceiver)
	s.Receiver = st.types.canon(s.Receiver)
	s.ReturnType = st.types.canon(s.ReturnType)
	s.DeclaredReturnType = st.types.canon(s.DeclaredReturnType)
	if s.Parameters != nil {
		ps := make([]Type, len(s.Parameters))
		for i := range s.Parameters {
			ps[i] = *st.types.canon(&s.Parameters[i])
		}
		s.Parameters = ps
	}
	if s.Targets != nil {
		ts := make([]Call, len(s.Targets))
		for i := range s.Targets {
			ts[i] = st.canonCall(&s.Targets[i])
		}
		s.Targets = ts
	}
	return s
}

// copyCallShape returns c with slices of its own, but its types and
// targets' types, which are interned.
func copyCallShape(c Call) Call {
	c.Declaration = copyID(c.Declaration)
	c.ImplicitReceiver = copyID(c.ImplicitReceiver)
	c.Parameters = slices.Clone(c.Parameters)
	c.ParameterNames = slices.Clone(c.ParameterNames)
	c.Varargs = slices.Clone(c.Varargs)
	c.ArgumentMapping = slices.Clone(c.ArgumentMapping)
	c.TypeParameterNames = slices.Clone(c.TypeParameterNames)
	c.Annotations = slices.Clone(c.Annotations)
	c.OwnerAnnotations = slices.Clone(c.OwnerAnnotations)
	c.PackageAnnotations = slices.Clone(c.PackageAnnotations)
	c.Crossinline = slices.Clone(c.Crossinline)
	c.Noinline = slices.Clone(c.Noinline)
	if c.Targets != nil {
		ts := make([]Call, len(c.Targets))
		for i, t := range c.Targets {
			ts[i] = copyCallShape(t)
		}
		c.Targets = ts
	}
	return c
}

func hashCall(h *maphash.Hash, c *Call) {
	h.WriteString(c.Name)
	if c.Declaration != nil {
		writeID(h, *c.Declaration)
	}
	writePtr(h, c.DeclaredReceiver)
	writePtr(h, c.DeclaredReturnType)
	writeInt(h, uint64(len(c.Parameters)))
	for _, n := range c.ParameterNames {
		h.WriteString(n)
	}
	for _, a := range c.Annotations {
		h.WriteString(a)
	}
	writeInt(h, uint64(len(c.Targets)))
	for i := range c.Targets {
		hashCall(h, &c.Targets[i])
	}
}

// sameCall reports whether a and b, whose types are interned, are equal.
func sameCall(a, b *Call) bool {
	if a.Name != b.Name || !sameID(a.Declaration, b.Declaration) || a.Extension != b.Extension ||
		a.DeclaredReceiver != b.DeclaredReceiver || a.Receiver != b.Receiver || !sameID(a.ImplicitReceiver, b.ImplicitReceiver) ||
		a.ReturnType != b.ReturnType || a.DeclaredReturnType != b.DeclaredReturnType || a.TypeParameters != b.TypeParameters ||
		a.Suspend != b.Suspend || a.Inline != b.Inline || a.Operator != b.Operator || a.FromJava != b.FromJava ||
		a.StableNames != b.StableNames || a.ValueInvocation != b.ValueInvocation ||
		!sameSlice(a.ParameterNames, b.ParameterNames) || !sameSlice(a.Varargs, b.Varargs) || !sameSlice(a.ArgumentMapping, b.ArgumentMapping) ||
		!sameSlice(a.TypeParameterNames, b.TypeParameterNames) || !sameSlice(a.Annotations, b.Annotations) ||
		!sameSlice(a.OwnerAnnotations, b.OwnerAnnotations) || !sameSlice(a.PackageAnnotations, b.PackageAnnotations) ||
		!sameSlice(a.Crossinline, b.Crossinline) || !sameSlice(a.Noinline, b.Noinline) ||
		(a.Parameters == nil) != (b.Parameters == nil) || len(a.Parameters) != len(b.Parameters) ||
		(a.Targets == nil) != (b.Targets == nil) || len(a.Targets) != len(b.Targets) {
		return false
	}
	for i := range a.Parameters {
		if !sameInterned(&a.Parameters[i], &b.Parameters[i]) {
			return false
		}
	}
	for i := range a.Targets {
		if !sameCall(&a.Targets[i], &b.Targets[i]) {
			return false
		}
	}
	return true
}

// sameInterned reports whether a and b are values of the same interned
// type: no two interned types are alike in their fields and their slices'
// identities.
func sameInterned(a, b *Type) bool {
	return a.Name == b.Name && a.Nullable == b.Nullable && a.MarkedNullable == b.MarkedNullable && a.Platform == b.Platform &&
		a.Parameter == b.Parameter && a.ParameterOwner == b.ParameterOwner && a.SAM == b.SAM && a.Suspend == b.Suspend &&
		a.Projection == b.Projection && sameArray(a.Arguments, b.Arguments) && sameArray(a.Supertypes, b.Supertypes)
}

// sameArray reports whether a and b are the same slice of one array.
func sameArray[T any](a, b []T) bool {
	return len(a) == len(b) && (a == nil) == (b == nil) && (len(a) == 0 || &a[0] == &b[0])
}

// sameSlice reports whether a and b are equal, nil apart from empty.
func sameSlice[T comparable](a, b []T) bool {
	return (a == nil) == (b == nil) && slices.Equal(a, b)
}

func sameID(a, b *ElementID) bool {
	return a == b || a != nil && b != nil && *a == *b
}

func writePtr[T any](h *maphash.Hash, p *T) {
	maphash.WriteComparable(h, p)
}

func writeID(h *maphash.Hash, id ElementID) {
	writeInt(h, uint64(id.Snapshot))
	writeInt(h, uint64(id.Node))
}

func writeInt(h *maphash.Hash, n uint64) {
	var b [8]byte
	for i := range b {
		b[i] = byte(n >> (8 * i))
	}
	h.Write(b[:])
}
