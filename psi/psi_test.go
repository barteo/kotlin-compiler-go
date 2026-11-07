package psi

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// checkTree verifies losslessness and the arena invariants lint relies on.
func checkTree(t testing.TB, s *Snapshot) {
	t.Helper()
	src := s.Source()
	n := s.NodeCount()
	if n < 1 || s.RootNode().NumericKind() != KindFILE {
		t.Fatalf("root = %v", s.RootNode().Kind())
	}
	root := s.nodes[1]
	if root.start != 0 || int(root.end) != len(src) || root.parent != 0 || int(root.subtreeEnd) != n+1 {
		t.Fatalf("root node %+v for %d bytes, %d nodes", root, len(src), n)
	}
	var text strings.Builder
	for id := NodeID(1); int(id) <= n; id++ {
		nd := &s.nodes[id]
		if nd.start > nd.end {
			t.Fatalf("node %d: range %d..%d", id, nd.start, nd.end)
		}
		if nd.subtreeEnd <= id || int(nd.subtreeEnd) > n+1 {
			t.Fatalf("node %d: subtreeEnd %d", id, nd.subtreeEnd)
		}
		if nd.leaf {
			if nd.firstChild != 0 || nd.subtreeEnd != id+1 || nd.start == nd.end {
				t.Fatalf("leaf %d: %+v", id, nd)
			}
			if int(nd.start) != text.Len() {
				t.Fatalf("leaf %d at %d, want %d", id, nd.start, text.Len())
			}
			text.WriteString(src[nd.start:nd.end])
		}
		if id > 1 {
			p := &s.nodes[nd.parent]
			if nd.parent == 0 || nd.parent >= id || nd.subtreeEnd > p.subtreeEnd {
				t.Fatalf("node %d: parent %d", id, nd.parent)
			}
			if nd.start < p.start || nd.end > p.end {
				t.Fatalf("node %d %d..%d outside parent %d..%d", id, nd.start, nd.end, p.start, p.end)
			}
		}
		// Children are contiguous, in order, and tile the subtree.
		next := id + 1
		pos := nd.start
		var prev NodeID
		for c := nd.firstChild; c != 0; c = s.nodes[c].nextSibling {
			cn := &s.nodes[c]
			if c != next || cn.parent != id || cn.prevSibling != prev || cn.start != pos {
				t.Fatalf("node %d: child %d (want id %d, start %d): %+v", id, c, next, pos, cn)
			}
			next, pos, prev = cn.subtreeEnd, cn.end, c
		}
		if !nd.leaf && (next != nd.subtreeEnd || pos != nd.end) {
			t.Fatalf("node %d: children end at %d/%d, node at %d/%d", id, next, pos, nd.subtreeEnd, nd.end)
		}
	}
	if text.String() != src {
		t.Fatal("leaf text does not reproduce the source")
	}
	for _, d := range s.Diagnostics() {
		if d.Code != "syntax_error" || d.Message == "" || d.Range.Start > d.Range.End || d.Range.End > len(src) {
			t.Fatalf("diagnostic %+v", d)
		}
	}
}

func TestCorpusInvariants(t *testing.T) {
	var p Parser
	for _, f := range corpusFiles(t) {
		text, err := os.ReadFile("../" + f)
		if err != nil {
			t.Fatal(err)
		}
		s := p.Parse(f, string(text))
		checkTree(t, s)
		if s.RootNode().Text() != string(text) {
			t.Fatalf("%s: root text differs", f)
		}
	}
}

func TestInvariantsOnOddInput(t *testing.T) {
	for _, src := range []string{
		"",
		" ",
		"\n\n",
		"/*",
		"/** doc",
		"\"",
		"\"${",
		"\"\"\"",
		"'",
		"`",
		"#!/bin/sh",
		"fun",
		"class A {",
		"}}}",
		")))",
		"fun f() = when { else -> }",
		"val x = 1 +",
		"a\rb\r\nc",
		"@",
		"package",
		"import a.",
		"fun f(x: *) {}",
		strings.Repeat("(", 600) + strings.Repeat(")", 600),
		strings.Repeat("{", 600),
		"\x00\xff\xfe",
		"val s = \"\U0001F600${x}\"",
	} {
		for _, path := range []string{"a.kt", "a.kts"} {
			s := Parse(path, src)
			checkTree(t, s)
		}
	}
}

func TestDeepNesting(t *testing.T) {
	src := "val x = " + strings.Repeat("(", maxNesting+1) + "1" + strings.Repeat(")", maxNesting+1)
	s := Parse("a.kt", src)
	checkTree(t, s)
	ds := s.Diagnostics()
	if len(ds) != 1 || ds[0].Message != "nesting too deep" {
		t.Fatalf("diagnostics = %v", ds)
	}
}

func TestZeroNodeRef(t *testing.T) {
	var n NodeRef
	if n.Valid() || n.ID() != 0 || n.NumericKind() != KindUnknown || n.Kind() != "" || n.Text() != "" ||
		n.Range() != (Range{}) || n.SubtreeEnd() != 0 || n.Parent().Valid() || n.FirstChild().Valid() ||
		n.NextSibling().Valid() || n.PreviousSibling().Valid() || n.ChildKind(KindFUN).Valid() ||
		n.Child("FUN").Valid() || n.IsLeaf() || n.IsTrivia() || n.ContainingFile() != nil ||
		n.Declaration().Valid() || n.Function().Valid() || n.Import().Valid() || n.Element().Valid() ||
		n.NameIdentifier().Valid() || n.Name() != "" || n.TypeReference().Valid() || n.ReturnType().Valid() ||
		n.ReceiverType().Valid() || n.Body().Valid() || n.Parameters() != nil || n.Path() != "" ||
		n.Alias() != "" || n.IsAllUnder() {
		t.Fatal("zero NodeRef is not inert")
	}
	WalkNodes(n, func(NodeRef) bool { t.Fatal("walked zero NodeRef"); return true })
	var s *Snapshot
	if s.Node(1).Valid() || s.NodeAt(0).Valid() {
		t.Fatal("nil snapshot")
	}
}

func TestAccessors(t *testing.T) {
	src := `package a.b

import x.y.Z
import x.y.* // all
import p.q as R

/** Doc. */
fun <T> String.` + "`my fun`" + `(a: Int, b: T): List<T> = listOf(b)

class C(val p: Int) {
    constructor() : this(1) {}
}
`
	s := Parse("a.kt", src)
	checkTree(t, s)
	if len(s.Diagnostics()) != 0 {
		t.Fatal(s.Diagnostics())
	}
	var imports []NodeRef
	WalkNodes(s.RootNode(), func(n NodeRef) bool {
		if n.Import().Valid() {
			imports = append(imports, n)
		}
		return true
	})
	if len(imports) != 3 {
		t.Fatalf("%d imports", len(imports))
	}
	if imports[0].Path() != "x.y.Z" || imports[0].IsAllUnder() || imports[0].Alias() != "" {
		t.Error("import 0", imports[0].Path())
	}
	if imports[1].Path() != "x.y" || !imports[1].IsAllUnder() {
		t.Error("import 1", imports[1].Path())
	}
	if imports[2].Path() != "p.q" || imports[2].Alias() != "R" {
		t.Error("import 2", imports[2].Path(), imports[2].Alias())
	}
	decls := s.DeclarationNodes()
	if len(decls) != 2 {
		t.Fatalf("%d declarations", len(decls))
	}
	fn := decls[0].Function()
	if fn.Name() != "my fun" || fn.ReceiverType().Text() != "String" || fn.ReturnType().Text() != "List<T>" ||
		fn.TypeReference().Text() != "List<T>" || fn.Body().Text() != "listOf(b)" {
		t.Errorf("fun: %q %q %q %q", fn.Name(), fn.ReceiverType().Text(), fn.ReturnType().Text(), fn.Body().Text())
	}
	if ps := fn.Parameters(); len(ps) != 2 || ps[0].Name() != "a" || ps[1].TypeReference().Text() != "T" {
		t.Errorf("parameters %v", ps)
	}
	if !strings.HasPrefix(fn.Text(), "/** Doc. */") {
		t.Errorf("KDoc not bound to fun: %q", fn.Text())
	}
	if c := fn.Child("KDOC"); !c.Valid() || !c.IsLeaf() {
		t.Error("KDOC child")
	}
	cls := decls[1]
	if cls.Name() != "C" || cls.NumericKind() != KindCLASS {
		t.Error("class", cls.Name())
	}
	off := strings.Index(src, "this(1)")
	leaf := s.NodeAt(off)
	if leaf.NumericKind() != KindTHIS_KEYWORD || !leaf.IsLeaf() {
		t.Errorf("NodeAt = %s", leaf.Kind())
	}
	if s.NodeAt(len(src)).NumericKind() != KindWHITE_SPACE {
		t.Error("NodeAt(end)")
	}
	if !s.NodeAt(strings.Index(src, "// all")).IsTrivia() {
		t.Error("comment is trivia")
	}
}

func TestObjectNames(t *testing.T) {
	src := "class A { companion object }\nclass B { companion object Named }\nval o = object {}\nobject O\n"
	s := Parse("a.kt", src)
	checkTree(t, s)
	var names []string
	WalkNodes(s.RootNode(), func(n NodeRef) bool {
		if n.NumericKind() == KindOBJECT_DECLARATION {
			names = append(names, n.Name())
		}
		return true
	})
	if want := []string{"Companion", "Named", "", "O"}; !slices.Equal(names, want) {
		t.Errorf("object names %q, want %q", names, want)
	}
}

func TestOffsets(t *testing.T) {
	src := "a\U0001F600b\r\nçd\n"
	s := Parse("a.kt", src)
	type want struct{ off, u16, line, col int }
	for _, w := range []want{
		{0, 0, 1, 1},
		{1, 1, 1, 2},
		{5, 3, 1, 4},  // b, after a surrogate pair
		{6, 4, 1, 5},  // \r
		{8, 6, 2, 1},  // ç
		{10, 7, 2, 2}, // d
		{12, 9, 3, 1}, // end
	} {
		u, l, c, err := s.OffsetPosition(w.off)
		if err != nil || u != w.u16 || l != w.line || c != w.col {
			t.Errorf("OffsetPosition(%d) = %d %d:%d %v, want %d %d:%d", w.off, u, l, c, err, w.u16, w.line, w.col)
		}
	}
	for _, off := range []int{-1, 2, 3, 4, 9, 13} {
		if _, err := s.UTF16Offset(off); err == nil {
			t.Errorf("UTF16Offset(%d) accepted", off)
		}
	}
	// Invalid bytes decode one rune each, as in a range loop.
	s = Parse("c.kt", "\xffa\xc0\x80😀")
	for off, u16 := range []int{0, 1, 2, 3, 4, -1, -1, -1, 6} {
		got, err := s.UTF16Offset(off)
		if u16 < 0 && (err == nil || err.Error() != fmt.Sprintf("invalid UTF-8 boundary: %d", off)) || u16 >= 0 && (err != nil || got != u16) {
			t.Errorf("UTF16Offset(%d) = %d %v, want %d", off, got, err, u16)
		}
	}
	// Checkpoints: a long non-ASCII source.
	long := strings.Repeat("ż\U0001F600x\n", 1000)
	s = Parse("b.kt", long)
	units := 0
	for i, r := range long {
		if i%37 == 0 {
			got, err := s.UTF16Offset(i)
			if err != nil || got != units {
				t.Fatalf("UTF16Offset(%d) = %d %v, want %d", i, got, err, units)
			}
		}
		units += utf16Len(r)
	}
	if got, _ := s.UTF16Offset(len(long)); got != units {
		t.Fatalf("end: %d, want %d", got, units)
	}
}

func TestParserReuse(t *testing.T) {
	var p Parser
	a := p.Parse("a.kt", "fun a() { val x = 1 }")
	b := p.Parse("b.kt", "class B")
	checkTree(t, a)
	checkTree(t, b)
	if a.ID() == b.ID() || a.RootNode().Text() != "fun a() { val x = 1 }" {
		t.Fatal("snapshots share state")
	}
	if p.RetainedBytes() == 0 {
		t.Fatal("RetainedBytes")
	}
}

func FuzzParse(f *testing.F) {
	for _, src := range []string{"fun f() {}", "class A<T>(val x: T) : B by c { init {} }", "val s = \"a${b}c\"", "@A fun f(x: *)"} {
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		done := make(chan struct{})
		timer := time.AfterFunc(5*time.Second, func() { panic(fmt.Sprintf("parse hangs on %q", src)) })
		defer timer.Stop()
		go func() {
			defer close(done)
			Parse("f.kt", src)
			Parse("f.kts", src)
		}()
		<-done
		checkTree(t, Parse("f.kt", src))
		checkTree(t, Parse("f.kts", src))
	})
}
