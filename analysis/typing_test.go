package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// TestSessionTyping replays typing each function body of programs of
// testdata/run: one session updates with the body cut at points from its
// start, as an editor that closed its brace sends it, and checks that the
// facts of the code outside the body, and of the body's statements before
// the cut, are the original's. KC_TYPING=all replays every program and
// cuts after every token; by default a sample of programs is cut at
// statement boundaries and in the middle of each statement.
func TestSessionTyping(t *testing.T) {
	cfg := withJDK(t)
	all := os.Getenv("KC_TYPING") == "all"
	mods := runModules(t)
	names := make([]string, 0, len(mods))
	for name := range mods {
		names = append(names, name)
	}
	slices.Sort(names)
	step := 14
	if testing.Short() {
		step = 70
	}
	var st typingStats
	for k, name := range names {
		if !all && k%step != 0 {
			continue
		}
		replayProgram(t, cfg, name, mods[name], all, &st)
	}
	t.Logf("%d programs, %d bodies, %d cuts, %d skipped (psi took the body's brace), %d failed", st.programs, st.bodies, st.cuts, st.skipped, st.failed)
}

type typingStats struct{ programs, bodies, cuts, skipped, failed int }

// typingFact is a fact by its element: the file and range, in the
// original's offsets, and its JSON, with the elements it names written
// as their files and ranges too.
type typingFact struct {
	file, start, end int
	json             string
}

// replayProgram replays typing each function body of the program in
// paths.
func replayProgram(t *testing.T, cfg Config, name string, paths []string, all bool, st *typingStats) {
	ctx := context.Background()
	first := parseFiles(t, paths)
	s := Open(cfg)
	defer s.Close()
	ref, err := s.Update(ctx, first)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if !ref.Complete() {
		return
	}
	st.programs++
	ids := map[string]uint32{}
	want := typingFacts(t, ref, first, nil, ids)
	failures := 0
	for i, snap := range first {
		src := snap.Source()
		for _, b := range bodies(snap) {
			st.bodies++
			lb, rb := b.ChildKind(psi.KindLBRACE).Range().Start, b.ChildKind(psi.KindRBRACE).Range().Start
			stmts := bodyStatements(b)
			for _, cut := range cuts(snap, b, stmts, all) {
				text := src[:lb+1] + src[lb+1:cut] + "\n" + src[rb:]
				newEnd := cut + 1 // where the original's src[rb:] starts
				snaps := slices.Clone(first)
				snaps[i] = psi.Parse(snap.Path(), text)
				if nb := snaps[i].NodeAt(lb).Parent(); nb.NumericKind() != psi.KindBLOCK || nb.ChildKind(psi.KindRBRACE).Range().Start != newEnd {
					st.skipped++
					continue
				}
				st.cuts++
				got, err := s.Update(ctx, snaps)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				delta := newEnd - rb
				shift := func(file, off int) int {
					if file == i && off >= newEnd {
						return off - delta
					}
					return off
				}
				have := typingFacts(t, got, snaps, shift, ids)
				if d := typingDiff(want, have, i, lb, rb, keptStatements(stmts, snaps[i], cut)); d != "" {
					st.failed++
					if failures++; failures <= 3 {
						t.Errorf("%s: %s, cut at %d:%s|: %s; diagnostics %v", name, snap.Path(), cut, tail(src[lb+1:cut], 60), d, got.Diagnostics())
					}
				}
			}
		}
	}
}

// bodies returns the block bodies of the functions of snap declared in
// the file or its classes, but for inline functions. A function of an
// object expression is part of the expression: an error in it is one in
// the expression's statement, or in the declaration whose '=' it follows.
// An inline function's body is part of what its callers see, as a
// contract is: kc reads it to check how they pass it lambdas, and numbers
// the anonymous classes it inlines into them.
func bodies(snap *psi.Snapshot) []psi.NodeRef {
	var out []psi.NodeRef
	for id := psi.NodeID(1); int(id) <= snap.NodeCount(); id++ {
		n := snap.Node(id)
		if n.NumericKind() != psi.KindBLOCK || n.Parent().NumericKind() != psi.KindFUN {
			continue
		}
		member := true
		for p := n.Parent().Parent(); p.Valid() && member; p = p.Parent() {
			switch p.NumericKind() {
			case psi.KindFILE, psi.KindCLASS_BODY, psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindENUM_ENTRY:
			default:
				member = false
			}
		}
		if !member || strings.Contains(n.Parent().ChildKind(psi.KindMODIFIER_LIST).Text(), "inline") {
			continue
		}
		if n.ChildKind(psi.KindLBRACE).Valid() && n.ChildKind(psi.KindRBRACE).Valid() {
			out = append(out, n)
		}
	}
	return out
}

// bodyStatements returns the statements of block b.
func bodyStatements(b psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	for c := b.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindLBRACE, psi.KindRBRACE, psi.KindSEMICOLON:
			continue
		}
		if !c.IsTrivia() {
			out = append(out, c)
		}
	}
	return out
}

// cuts returns the offsets to cut body b at: after every token, or at
// the start, at each statement's end and in its middle. A contract, which
// is the function's signature as much as its parameters, is kept whole.
func cuts(snap *psi.Snapshot, b psi.NodeRef, stmts []psi.NodeRef, all bool) []int {
	lb := b.ChildKind(psi.KindLBRACE).Range().End
	out := []int{lb}
	if len(stmts) > 0 {
		if t := stmts[0].Text(); strings.HasPrefix(t, "contract") || strings.HasPrefix(t, "kotlin.contracts") {
			out = []int{stmts[0].Range().End}
			stmts = stmts[1:]
		}
	}
	for _, s := range stmts {
		var ends []int
		for id := s.ID(); id < s.SubtreeEnd(); id++ {
			if n := snap.Node(id); n.IsLeaf() && !n.IsTrivia() && n.Range().End > n.Range().Start {
				ends = append(ends, n.Range().End)
			}
		}
		switch {
		case all:
			out = append(out, ends...)
		case len(ends) > 2:
			out = append(out, ends[len(ends)/2], s.Range().End)
		default:
			out = append(out, s.Range().End)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// keptStatements returns the ranges of the statements of stmts that end
// before cut and are statements of the cut file's body as they were.
func keptStatements(stmts []psi.NodeRef, cut *psi.Snapshot, at int) []psi.Range {
	var out []psi.Range
	for _, s := range stmts {
		r := s.Range()
		if r.End > at {
			break
		}
		for n := cut.NodeAt(r.Start); n.Valid(); n = n.Parent() {
			if n.Range() == r && n.NumericKind() == s.NumericKind() && n.Parent().NumericKind() == psi.KindBLOCK {
				out = append(out, r)
				break
			}
			if n.Range().Start != r.Start {
				break
			}
		}
	}
	return out
}

// typingFacts returns res's facts by their elements, written in the
// original's offsets by shift, if it isn't nil; ids numbers the elements
// facts and symbols name across results.
func typingFacts(t *testing.T, res *Result, snaps []*psi.Snapshot, shift func(file, off int) int, ids map[string]uint32) map[string]typingFact {
	t.Helper()
	if shift == nil {
		shift = func(_, off int) int { return off }
	}
	files := map[psi.SnapshotID]int{}
	for i, s := range snaps {
		files[s.ID()] = i
	}
	where := func(e ElementID) (string, int, int, int) {
		i := files[e.Snapshot]
		n := snaps[i].Node(e.Node)
		r := n.Range()
		start, end := shift(i, r.Start), shift(i, r.End)
		return fmt.Sprintf("%d:%d:%d:%s", i, start, end, n.Kind()), i, start, end
	}
	canon := func(e *ElementID) {
		k, _, _, _ := where(*e)
		id, ok := ids[k]
		if !ok {
			id = uint32(len(ids) + 1)
			ids[k] = id
		}
		*e = ElementID{Node: psi.NodeID(id)}
	}
	d := res.Data()
	syms := map[uint32]string{}
	for _, s := range d.Symbols {
		id := s.ID
		s.ID = 0
		symbolIDs(&s, canon)
		syms[id] = mustJSON(t, s)
	}
	out := map[string]typingFact{}
	for _, f := range d.Facts {
		k, i, start, end := where(f.Element)
		c := copyFact(f)
		factIDs(&c, canon)
		sym, lambda := syms[c.Symbol], []string{}
		for _, p := range c.LambdaParameters {
			lambda = append(lambda, syms[p])
		}
		c.Element, c.Symbol, c.LambdaParameters = ElementID{}, 0, nil
		out[k] = typingFact{i, start, end, fmt.Sprintf("%s sym=%s lambda=%q", mustJSON(t, c), sym, lambda)}
	}
	return out
}

func mustJSON(t *testing.T, v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// typingDiff returns how have differs from want outside the body of file
// at [lb, rb], and in its kept statements, or "".
func typingDiff(want, have map[string]typingFact, file, lb, rb int, kept []psi.Range) string {
	inBody := func(f typingFact) bool { return f.file == file && f.start >= lb && f.end <= rb+1 }
	inKept := func(f typingFact) bool {
		for _, r := range kept {
			if f.file == file && f.start >= r.Start && f.end <= r.End {
				return true
			}
		}
		return false
	}
	var diffs []string
	for k, w := range want {
		if inBody(w) && !inKept(w) {
			continue
		}
		h, ok := have[k]
		switch {
		case !ok:
			diffs = append(diffs, "lost "+k)
		case h.json != w.json:
			diffs = append(diffs, fmt.Sprintf("%s:\n got %s\nwant %s", k, h.json, w.json))
		}
	}
	for k, h := range have {
		if _, ok := want[k]; !ok && (!inBody(h) || inKept(h)) {
			diffs = append(diffs, "extra "+k)
		}
	}
	if len(diffs) == 0 {
		return ""
	}
	slices.Sort(diffs)
	return fmt.Sprintf("%d differences, the first %s", len(diffs), diffs[0])
}

func tail(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "⏎")
	if r := []rune(s); len(r) > n {
		return "…" + string(r[len(r)-n:])
	}
	return s
}
