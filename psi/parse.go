package psi

import (
	"strings"
	"time"
)

// ParseStages reports where ParseMeasured spent its time.
type ParseStages struct {
	Lex       time.Duration
	Events    time.Duration // parsing into the production list
	Publish   time.Duration // building the arena
	Positions time.Duration // line and UTF-16 tables
}

// maxNesting bounds bracket nesting. The parser is recursive; deeper input
// is kept as a flat, lossless tree with a diagnostic instead.
const maxNesting = 512

// Parser parses Kotlin source. Its zero value is ready to use. A Parser
// reuses its buffers between calls and is not safe for concurrent use;
// snapshots never share its buffers.
type Parser struct {
	kinds  []Kind
	starts []int32
	lexed  []Kind
	nodes  []node
}

// Parse parses text as the file at path. It never fails: syntax errors are
// reported by Snapshot.Diagnostics. It is safe for concurrent use.
func Parse(path, text string) *Snapshot {
	var p Parser
	return p.Parse(path, text)
}

// Parse parses text as the file at path.
func (p *Parser) Parse(path, text string) *Snapshot {
	s, _ := p.ParseMeasured(path, text)
	return s
}

// RetainedBytes returns the size of the buffers the Parser keeps.
func (p *Parser) RetainedBytes() int {
	return cap(p.kinds)*2 + cap(p.starts)*4 + cap(p.lexed)*2 + cap(p.nodes)*int(nodeSize)
}

const nodeSize = 36

// ParseMeasured is Parse that also reports stage timings.
func (p *Parser) ParseMeasured(path, text string) (*Snapshot, ParseStages) {
	var st ParseStages
	t0 := time.Now()
	p.kinds, p.starts = tokenize(text, p.kinds[:0], p.starts[:0])
	p.lexed = append(p.lexed[:0], p.kinds...)
	t1 := time.Now()
	st.Lex = t1.Sub(t0)

	w := treeWriter{nodes: append(p.nodes[:0], node{}), lexed: p.lexed}
	if nesting(p.kinds) > maxNesting {
		w.open(KindFILE, 0)
		b := newBuilder(text, p.kinds, p.starts, false)
		w.leaves(b, 0, b.n)
		w.close(int32(len(text)))
		w.syntaxError(0, int32(len(text)), "nesting too deep")
		st.Events = time.Since(t1)
	} else {
		b := newBuilder(text, p.kinds, p.starts, true)
		kp := createForTopLevel(b)
		if strings.HasSuffix(path, ".kts") {
			kp.parseScript()
		} else {
			kp.parseFile()
		}
		t2 := time.Now()
		st.Events = t2.Sub(t1)
		w.emit(b)
		st.Publish = time.Since(t2)
	}
	p.nodes = w.nodes[:0]
	nodes := make([]node, len(w.nodes))
	copy(nodes, w.nodes)
	t3 := time.Now()
	s := newSnapshot(path, text, nodes, w.diags)
	st.Positions = time.Since(t3)
	return s, st
}

// nesting returns the maximum bracket nesting of a token stream.
func nesting(kinds []Kind) int {
	d, m := 0, 0
	for _, k := range kinds {
		switch k {
		case KindLPAR, KindLBRACE, KindLBRACKET, KindLONG_TEMPLATE_ENTRY_START:
			d++
			m = max(m, d)
		case KindRPAR, KindRBRACE, KindRBRACKET, KindLONG_TEMPLATE_ENTRY_END:
			d--
		}
	}
	return m
}
