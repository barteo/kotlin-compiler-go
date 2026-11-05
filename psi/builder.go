package psi

// builder is a port of IntelliJ's PsiBuilderImpl combined with Kotlin's
// SemanticWhitespaceAwarePsiBuilderImpl (see THIRD_PARTY_NOTICES.md).
// The parser records a production list of start, done and error entries
// over the token array; balance and emit turn it into a tree.

// binderID names a WhitespacesAndCommentsBinder.
type binderID uint8

const (
	defaultLeftBinder  binderID = iota // bind no leading trivia
	defaultRightBinder                 // bind no trailing trivia
	precedingCommentsBinder
	precedingDocCommentsBinder
	trailingCommentsBinder
	precedingAllCommentsBinder
	trailingAllCommentsBinder
	doNotBindAnything
	bindFirstShebangWithWhitespaceOnly
	precedingAllBinder // BindAll(false)
	trailingAllBinder  // BindAll(true)
)

const (
	prodStart uint8 = iota
	prodDone
	prodError
)

// prod is one production entry.
type prod struct {
	m    int32 // marker index
	lex  int32 // lexeme index
	kind uint8
}

type markerData struct {
	typ       Kind
	left      binderID
	right     binderID
	collapsed bool
	lazy      bool // collapsed lazily; reparsed when the tree is built
	done      bool
	start     int32  // lexeme index of the start production
	msg       string // error message for ERROR_ELEMENT markers and error items
}

// Marker is PsiBuilder.Marker.
type Marker struct {
	b *builder
	i int32
}

type builder struct {
	src    string
	types  []Kind  // token kinds; remapCurrentToken writes here
	starts []int32 // len(types)+1 token start offsets
	n      int
	base0  int // index of types[0] in the whole file's token array

	cur     int
	checked bool // whitespace before cur has been skipped

	markers []markerData
	prods   []prod

	newlines []bool
	join     []bool

	// lazyKind selects the reparse entry point for lazily collapsed nodes.
	lazy bool
}

func newBuilder(src string, types []Kind, starts []int32, lazy bool) *builder {
	return &builder{
		src: src, types: types, starts: starts, n: len(types),
		newlines: []bool{true}, join: []bool{true}, lazy: lazy,
	}
}

func isWhitespaceOrComment(k Kind) bool {
	return k == KindWHITE_SPACE || k == KindEOL_COMMENT || k == KindBLOCK_COMMENT || k == kDocComment || k == KindSHEBANG_COMMENT
}

// ---- PsiBuilderImpl ----

func (b *builder) skipWhitespace() {
	for b.cur < b.n && isWhitespaceOrComment(b.types[b.cur]) {
		b.cur++
	}
}

func (b *builder) rawEOF() bool {
	if !b.checked {
		b.checked = true
		b.skipWhitespace()
	}
	return b.cur >= b.n
}

func (b *builder) rawTokenType() Kind {
	if b.rawEOF() {
		return kNone
	}
	return b.types[b.cur]
}

func (b *builder) rawTokenText() (string, bool) {
	if b.rawEOF() {
		return "", false
	}
	return b.src[b.starts[b.cur]:b.starts[b.cur+1]], true
}

func (b *builder) rawAdvance() {
	if b.rawEOF() {
		return
	}
	b.checked = false
	b.cur++
}

func (b *builder) rawLookAhead(steps int) Kind {
	if b.rawEOF() {
		return kNone
	}
	cur := b.cur
	for steps > 0 {
		cur++
		for cur < b.n && isWhitespaceOrComment(b.types[cur]) {
			cur++
		}
		steps--
	}
	if cur < b.n {
		return b.types[cur]
	}
	return kNone
}

func (b *builder) rawLookup(steps int) Kind {
	cur := b.cur + steps
	if cur >= 0 && cur < b.n {
		return b.types[cur]
	}
	return kNone
}

func (b *builder) rawTokenTypeStart(steps int) int {
	cur := b.cur + steps
	if cur < 0 {
		return -1
	}
	if cur >= b.n {
		return int(b.starts[b.n])
	}
	return int(b.starts[cur])
}

func (b *builder) rawTokenIndex() int { return b.cur }

func (b *builder) currentOffset() int {
	if b.rawEOF() {
		return int(b.starts[b.n])
	}
	return int(b.starts[b.cur])
}

func (b *builder) remapCurrentToken(k Kind) {
	if b.cur < b.n {
		b.types[b.cur] = k
	}
}

func (b *builder) originalText() string { return b.src }

func (b *builder) mark() Marker {
	if len(b.prods) > 0 {
		b.skipWhitespace()
	}
	i := int32(len(b.markers))
	b.markers = append(b.markers, markerData{left: defaultLeftBinder, right: defaultRightBinder, start: int32(b.cur)})
	b.prods = append(b.prods, prod{m: i, lex: int32(b.cur), kind: prodStart})
	return Marker{b, i}
}

func (b *builder) error(msg string) {
	if l := len(b.prods); l > 0 {
		if last := b.prods[l-1]; last.kind == prodError && int(last.lex) == b.cur {
			return
		}
	}
	i := int32(len(b.markers))
	b.markers = append(b.markers, markerData{typ: KindERROR_ELEMENT, msg: msg, right: defaultRightBinder, done: true})
	b.prods = append(b.prods, prod{m: i, lex: int32(b.cur), kind: prodError})
}

// isEmpty reports whether the tokens [from, to) are all trivia.
func (b *builder) isEmpty(from, to int) bool {
	for i := from; i < to; i++ {
		if !isWhitespaceOrComment(b.types[i]) {
			return false
		}
	}
	return true
}

func (b *builder) startIndex(m int32) int {
	for i := len(b.prods) - 1; i >= 0; i-- {
		if p := b.prods[i]; p.m == m && p.kind == prodStart {
			return i
		}
	}
	panic("psi: marker not in production")
}

// Done is Marker.done.
func (m Marker) Done(k Kind) {
	d := &m.b.markers[m.i]
	d.typ = k
	d.done = true
	// PsiBuilderImpl.processDone: an empty left-bound element binds to the
	// previous token. The left-bound types are the two Kotlin
	// KtLeftBoundNodeTypes and, since the IntelliJ platform bundled with
	// Kotlin 2.4, TokenType.ERROR_ELEMENT.
	if (k == KindCONSTRUCTOR_DELEGATION_CALL || k == KindCONSTRUCTOR_DELEGATION_REFERENCE || k == KindERROR_ELEMENT) && m.b.isEmpty(int(d.start), m.b.cur) {
		d.left = defaultRightBinder
	}
	m.b.prods = append(m.b.prods, prod{m: m.i, lex: int32(m.b.cur), kind: prodDone})
}

// Collapse is Marker.collapse: the marker becomes a leaf.
func (m Marker) Collapse(k Kind) {
	m.Done(k)
	m.b.markers[m.i].collapsed = true
	if m.b.lazy && (k == KindBLOCK || k == KindLAMBDA_EXPRESSION) {
		m.b.markers[m.i].lazy = true
	}
}

// Error is Marker.error: done as an ERROR_ELEMENT with a message.
func (m Marker) Error(msg string) {
	m.Done(KindERROR_ELEMENT)
	m.b.markers[m.i].msg = msg
}

// Drop is Marker.drop.
func (m Marker) Drop() {
	i := m.b.startIndex(m.i)
	m.b.prods = append(m.b.prods[:i], m.b.prods[i+1:]...)
}

// RollbackTo is Marker.rollbackTo.
func (m Marker) RollbackTo() {
	i := m.b.startIndex(m.i)
	m.b.cur = int(m.b.prods[i].lex)
	m.b.checked = true
	m.b.prods = m.b.prods[:i]
}

// Precede is Marker.precede.
func (m Marker) Precede() Marker {
	b := m.b
	i := b.startIndex(m.i)
	n := int32(len(b.markers))
	b.markers = append(b.markers, markerData{left: defaultLeftBinder, right: defaultRightBinder, start: b.prods[i].lex})
	b.prods = append(b.prods, prod{})
	copy(b.prods[i+1:], b.prods[i:])
	b.prods[i] = prod{m: n, lex: b.prods[i+1].lex, kind: prodStart}
	return Marker{b, n}
}

// SetCustomEdgeTokenBinders is Marker.setCustomEdgeTokenBinders.
func (m Marker) SetCustomEdgeTokenBinders(left, right binderID) {
	m.b.markers[m.i].left = left
	m.b.markers[m.i].right = right
}

// ---- SemanticWhitespaceAwarePsiBuilderImpl ----

func (b *builder) newlineBeforeCurrentToken() bool {
	if !b.newlines[len(b.newlines)-1] {
		return false
	}
	if b.rawEOF() {
		return true
	}
	for i := 1; b.cur-i >= 0; i++ {
		t := b.types[b.cur-i]
		if t == KindBLOCK_COMMENT || t == kDocComment || t == KindEOL_COMMENT || t == KindSHEBANG_COMMENT {
			continue
		}
		if t != KindWHITE_SPACE {
			break
		}
		for j := b.starts[b.cur-i]; j < b.starts[b.cur-i+1]; j++ {
			if b.src[j] == '\n' {
				return true
			}
		}
	}
	return false
}

func (b *builder) disableNewlines()      { b.newlines = append(b.newlines, false) }
func (b *builder) enableNewlines()       { b.newlines = append(b.newlines, true) }
func (b *builder) restoreNewlinesState() { b.newlines = b.newlines[:len(b.newlines)-1] }

func (b *builder) disableJoiningComplexTokens() { b.join = append(b.join, false) }
func (b *builder) enableJoiningComplexTokens()  { b.join = append(b.join, true) }
func (b *builder) restoreJoiningComplexTokensState() {
	b.join = b.join[:len(b.join)-1]
}

func (b *builder) joining() bool { return b.join[len(b.join)-1] }

func isComplexToken(k Kind) bool {
	return k == KindSAFE_ACCESS || k == KindELVIS || k == KindEXCLEXCL
}

func (b *builder) joinedTokenType(raw Kind, steps int) Kind {
	switch raw {
	case KindQUEST:
		switch b.rawLookup(steps) {
		case KindDOT:
			return KindSAFE_ACCESS
		case KindCOLON:
			return KindELVIS
		}
	case KindEXCL:
		if b.rawLookup(steps) == KindEXCL {
			return KindEXCLEXCL
		}
	}
	return raw
}

func (b *builder) tokenType() Kind {
	if !b.joining() {
		return b.rawTokenType()
	}
	return b.joinedTokenType(b.rawTokenType(), 1)
}

func (b *builder) advanceLexer() {
	if !b.joining() {
		b.rawAdvance()
		return
	}
	if t := b.tokenType(); isComplexToken(t) {
		m := b.mark()
		b.rawAdvance()
		b.rawAdvance()
		m.Collapse(t)
	} else {
		b.rawAdvance()
	}
}

func (b *builder) tokenText() (string, bool) {
	if !b.joining() {
		return b.rawTokenText()
	}
	switch b.tokenType() {
	case KindELVIS:
		return "?:", true
	case KindSAFE_ACCESS:
		return "?.", true
	}
	return b.rawTokenText()
}

func (b *builder) lookAhead(steps int) Kind {
	if !b.joining() {
		return b.rawLookAhead(steps)
	}
	if isComplexToken(b.tokenType()) {
		return b.rawLookAhead(steps + 1)
	}
	return b.joinedTokenType(b.rawLookAhead(steps), 2)
}

func (b *builder) eof() bool { return b.rawEOF() }

// ---- builder views ----

// psiBuilder is SemanticWhitespaceAwarePsiBuilder. The truncated and
// by-clause adapters override parts of it.
type psiBuilder interface {
	base() *builder
	mark() Marker
	error(msg string)
	tokenType() Kind
	tokenText() (string, bool)
	advanceLexer()
	eof() bool
	lookAhead(steps int) Kind
	rawLookup(steps int) Kind
	rawTokenTypeStart(steps int) int
	rawTokenIndex() int
	currentOffset() int
	remapCurrentToken(k Kind)
	originalText() string
	newlineBeforeCurrentToken() bool
	disableNewlines()
	enableNewlines()
	restoreNewlinesState()
	disableJoiningComplexTokens()
	enableJoiningComplexTokens()
	restoreJoiningComplexTokensState()
}

func (b *builder) base() *builder { return b }

// truncatedBuilder is TruncatedSemanticWhitespaceAwarePsiBuilder: input
// ends at eofPos.
type truncatedBuilder struct {
	psiBuilder
	eofPos int
}

func (t *truncatedBuilder) beyond(off int) bool { return t.eofPos >= 0 && off >= t.eofPos }

func (t *truncatedBuilder) eof() bool {
	return t.psiBuilder.eof() || t.beyond(t.currentOffset())
}

func (t *truncatedBuilder) tokenText() (string, bool) {
	if t.eof() {
		return "", false
	}
	return t.psiBuilder.tokenText()
}

func (t *truncatedBuilder) tokenType() Kind {
	if t.eof() {
		return kNone
	}
	return t.psiBuilder.tokenType()
}

func (t *truncatedBuilder) lookAhead(steps int) Kind {
	if t.eof() {
		return kNone
	}
	cur := 0
	for steps > 0 {
		cur++
		r := t.rawLookup(cur)
		for r != kNone && isWhitespaceOrComment(r) {
			cur++
			r = t.rawLookup(cur)
		}
		steps--
	}
	if t.beyond(t.rawTokenTypeStart(cur)) {
		return kNone
	}
	return t.rawLookup(cur)
}

// byClauseBuilder is SemanticWhitespaceAwarePsiBuilderForByClause.
type byClauseBuilder struct {
	psiBuilder
	stackSize int
}

func (c *byClauseBuilder) disableNewlines()      { c.psiBuilder.disableNewlines(); c.stackSize++ }
func (c *byClauseBuilder) enableNewlines()       { c.psiBuilder.enableNewlines(); c.stackSize++ }
func (c *byClauseBuilder) restoreNewlinesState() { c.psiBuilder.restoreNewlinesState(); c.stackSize-- }
