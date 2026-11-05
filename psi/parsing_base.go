package psi

// Port of AbstractKotlinParsing and the token-stream pattern helpers from
// Kotlin 2.4.21 (see THIRD_PARTY_NOTICES.md). Java's overloads become
// separate methods: expect(k), expectMsg(k, msg), expectRS(k, msg, rs).

type abstractParsing struct {
	b      psiBuilder
	isLazy bool
}

func (p *abstractParsing) getLastToken() Kind {
	i := 1
	cur := p.b.currentOffset()
	for i <= cur && whiteSpaceOrCommentSet.has(p.b.rawLookup(-i)) {
		i++
	}
	return p.b.rawLookup(-i)
}

func (p *abstractParsing) mark() Marker         { return p.b.mark() }
func (p *abstractParsing) error(msg string)     { p.b.error(msg) }
func (p *abstractParsing) eof() bool            { return p.b.eof() }
func (p *abstractParsing) advance()             { p.b.advanceLexer() }
func (p *abstractParsing) tt() Kind             { return p.b.tokenType() }
func (p *abstractParsing) lookahead(k int) Kind { return p.b.lookAhead(k) }

func (p *abstractParsing) advanceN(n int) {
	for i := 0; i < n; i++ {
		p.advance()
	}
}

func (p *abstractParsing) advanceAt(k Kind) { p.b.advanceLexer() }

func (p *abstractParsing) tokenText() string {
	s, _ := p.b.tokenText()
	return s
}

func (p *abstractParsing) expectMsg(k Kind, msg string) bool { return p.expectRS(k, msg, nil) }

func (p *abstractParsing) expectRS(k Kind, msg string, rs *tokenSet) bool {
	if p.expect(k) {
		return true
	}
	p.errorWithRecovery(msg, rs)
	return false
}

func (p *abstractParsing) expect(k Kind) bool {
	if p.at(k) {
		p.advance()
		return true
	}
	if t, ok := p.b.tokenText(); k == KindIDENTIFIER && ok && t == "`" {
		p.advance()
	}
	return false
}

func (p *abstractParsing) expectNoAdvance(k Kind, msg string) {
	if p.at(k) {
		p.advance()
		return
	}
	p.error(msg)
}

func (p *abstractParsing) errorWithRecovery(msg string, rs *tokenSet) {
	t := p.tt()
	if rs == nil || rs.has(t) || t == KindLBRACE || t == KindRBRACE ||
		rs.has(kEOLOrSemicolon) && (p.eof() || t == KindSEMICOLON || p.b.newlineBeforeCurrentToken()) {
		p.error(msg)
	} else {
		p.errorAndAdvance(msg)
	}
}

func (p *abstractParsing) errorAndAdvance(msg string) { p.errorAndAdvanceN(msg, 1) }

func (p *abstractParsing) errorAndAdvanceN(msg string, n int) {
	m := p.mark()
	p.advanceN(n)
	m.Error(msg)
}

// _at is the side-effect-free at.
func (p *abstractParsing) _at(k Kind) bool {
	return p.tokenMatches(p.tt(), k)
}

func (p *abstractParsing) tokenMatches(t, k Kind) bool {
	if t == k {
		return true
	}
	if k == kEOLOrSemicolon {
		if p.eof() || t == KindSEMICOLON || p.b.newlineBeforeCurrentToken() {
			return true
		}
	}
	return false
}

func (p *abstractParsing) at(k Kind) bool {
	if p._at(k) {
		return true
	}
	t := p.tt()
	if t == KindIDENTIFIER && isSoftKeyword(k) {
		if s, ok := p.b.tokenText(); ok && keywordValue[k] == s {
			p.b.remapCurrentToken(k)
			return true
		}
	}
	if k == KindIDENTIFIER && isSoftKeyword(t) {
		p.b.remapCurrentToken(KindIDENTIFIER)
		return true
	}
	return false
}

// _atSet is the side-effect-free atSet.
func (p *abstractParsing) _atSet(s tokenSet) bool {
	t := p.tt()
	if s.has(t) {
		return true
	}
	if s.has(kEOLOrSemicolon) {
		if p.eof() || t == KindSEMICOLON || p.b.newlineBeforeCurrentToken() {
			return true
		}
	}
	return false
}

func (p *abstractParsing) atSet(s tokenSet) bool {
	if p._atSet(s) {
		return true
	}
	t := p.tt()
	if t == KindIDENTIFIER {
		if text, ok := p.b.tokenText(); ok {
			if kw, ok := softKeywordTexts[text]; ok && s.has(kw) {
				p.b.remapCurrentToken(kw)
				return true
			}
		}
	} else if s.has(KindIDENTIFIER) && isSoftKeyword(t) {
		p.b.remapCurrentToken(KindIDENTIFIER)
		return true
	}
	return false
}

func (p *abstractParsing) consumeIf(k Kind) bool {
	if p.at(k) {
		p.advance()
		return true
	}
	return false
}

func (p *abstractParsing) skipUntil(s tokenSet) {
	stopAtEOL := s.has(kEOLOrSemicolon)
	for !p.eof() && !s.has(p.tt()) && !(stopAtEOL && p.at(kEOLOrSemicolon)) {
		p.advance()
	}
}

func (p *abstractParsing) errorUntil(msg string, s tokenSet) {
	m := p.mark()
	p.skipUntil(s)
	m.Error(msg)
}

func errorIf(m Marker, cond bool, msg string) {
	if cond {
		m.Error(msg)
	} else {
		m.Drop()
	}
}

// optionalMarker is AbstractKotlinParsing.OptionalMarker.
type optionalMarker struct {
	p      *abstractParsing
	m      Marker
	ok     bool
	offset int
}

func (p *abstractParsing) newOptionalMarker(actuallyMark bool) optionalMarker {
	o := optionalMarker{p: p, ok: actuallyMark}
	if actuallyMark {
		o.m = p.mark()
	}
	o.offset = p.b.currentOffset()
	return o
}

func (o optionalMarker) done(k Kind) {
	if o.ok {
		o.m.Done(k)
	}
}

func (o optionalMarker) error(msg string) {
	if !o.ok {
		return
	}
	if o.offset == o.p.b.currentOffset() {
		o.m.Drop() // no empty errors
	} else {
		o.m.Error(msg)
	}
}

func (o optionalMarker) drop() {
	if o.ok {
		o.m.Drop()
	}
}

func (p *abstractParsing) eol() bool { return p.b.newlineBeforeCurrentToken() || p.eof() }

func closeDeclarationWithCommentBinders(m Marker, k Kind, precedingNonDocComments bool) {
	m.Done(k)
	left := precedingDocCommentsBinder
	if precedingNonDocComments {
		left = precedingCommentsBinder
	}
	m.SetCustomEdgeTokenBinders(left, trailingCommentsBinder)
}

// ---- token stream patterns ----

// tokenStreamPredicate is TokenStreamPredicate.
type tokenStreamPredicate func(topLevel bool) bool

func (a tokenStreamPredicate) or(b tokenStreamPredicate) tokenStreamPredicate {
	return func(topLevel bool) bool { return a(topLevel) || b(topLevel) }
}

// tokenStreamPattern is TokenStreamPattern.
type tokenStreamPattern interface {
	processToken(offset int, topLevel bool) bool
	result() int
	isTopLevel(openAngleBrackets, openBrackets, openBraces, openParentheses int) bool
	handleUnmatchedClosing(k Kind) bool
	reset()
}

// abstractTokenStreamPattern is AbstractTokenStreamPattern.
type abstractTokenStreamPattern struct{ lastOccurrence int }

func (a *abstractTokenStreamPattern) fail()       { a.lastOccurrence = -1 }
func (a *abstractTokenStreamPattern) result() int { return a.lastOccurrence }
func (a *abstractTokenStreamPattern) reset()      { a.lastOccurrence = -1 }
func (a *abstractTokenStreamPattern) isTopLevel(angle, brackets, braces, parens int) bool {
	return braces == 0 && brackets == 0 && parens == 0 && angle == 0
}
func (a *abstractTokenStreamPattern) handleUnmatchedClosing(Kind) bool { return false }

// firstBefore is FirstBefore.
type firstBefore struct {
	abstractTokenStreamPattern
	lookFor, stopAt tokenStreamPredicate
}

func newFirstBefore(lookFor, stopAt tokenStreamPredicate) *firstBefore {
	return &firstBefore{abstractTokenStreamPattern{-1}, lookFor, stopAt}
}

func (f *firstBefore) processToken(offset int, topLevel bool) bool {
	if f.lookFor(topLevel) {
		f.lastOccurrence = offset
		return true
	}
	return f.stopAt(topLevel)
}

// lastBefore is LastBefore.
type lastBefore struct {
	abstractTokenStreamPattern
	lookFor, stopAt              tokenStreamPredicate
	dontStopRightAfterOccurrence bool
	previousLookForResult        bool
}

func newLastBefore(lookFor, stopAt tokenStreamPredicate, dontStopRightAfterOccurrence bool) *lastBefore {
	return &lastBefore{abstractTokenStreamPattern: abstractTokenStreamPattern{-1}, lookFor: lookFor, stopAt: stopAt,
		dontStopRightAfterOccurrence: dontStopRightAfterOccurrence}
}

func (l *lastBefore) processToken(offset int, topLevel bool) bool {
	r := l.lookFor(topLevel)
	if r {
		l.lastOccurrence = offset
	}
	if l.stopAt(topLevel) {
		if topLevel && (!l.dontStopRightAfterOccurrence || !l.previousLookForResult) {
			return true
		}
	}
	l.previousLookForResult = r
	return false
}

func (l *lastBefore) reset() {
	l.lastOccurrence = -1
	l.previousLookForResult = false
}

// atPred is AbstractKotlinParsing.At.
func (p *abstractParsing) atPred(lookFor Kind, topLevelOnly bool) tokenStreamPredicate {
	return func(topLevel bool) bool { return (topLevel || !topLevelOnly) && p.at(lookFor) }
}

// atSetPred is AbstractKotlinParsing.AtSet.
func (p *abstractParsing) atSetPred(lookFor, topLevelOnly tokenSet) tokenStreamPredicate {
	return func(topLevel bool) bool { return (topLevel || !p.atSet(topLevelOnly)) && p.atSet(lookFor) }
}

func (p *abstractParsing) matchTokenStreamPredicate(pattern tokenStreamPattern) int {
	pos := p.mark()
	var opens []Kind
	angle, braces, parens, brackets := 0, 0, 0, 0
	for !p.eof() {
		if pattern.processToken(p.b.currentOffset(), pattern.isTopLevel(angle, brackets, braces, parens)) {
			break
		}
		switch p.tt() {
		case KindLPAR:
			parens++
			opens = append(opens, KindLPAR)
		case KindLT:
			angle++
			opens = append(opens, KindLT)
		case KindLBRACE:
			braces++
			opens = append(opens, KindLBRACE)
		case KindLBRACKET:
			brackets++
			opens = append(opens, KindLBRACKET)
		case KindRPAR:
			parens--
			matched := false
			if len(opens) > 0 {
				top := opens[len(opens)-1]
				opens = opens[:len(opens)-1]
				matched = top == KindLPAR
			}
			if !matched && pattern.handleUnmatchedClosing(KindRPAR) {
				// Java's "break" here only leaves the switch.
			}
		case KindGT:
			angle--
		case KindRBRACE:
			braces--
		case KindRBRACKET:
			brackets--
		}
		p.advance()
	}
	pos.RollbackTo()
	return pattern.result()
}
