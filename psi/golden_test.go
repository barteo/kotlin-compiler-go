package psi

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The goldens come from tools/psidump, which parses with stock Kotlin:
//
//	psi/testdata/trees/*.tree  full dumps of the fixtures in psi/testdata/src
//	psi/testdata/corpus.sum    sha256 of the dump of every testdata/**/*.kt
//
// Dumps list one node per line in preorder, as
// "<depth> <KIND> <start> <end>" with UTF-16 offsets, then !"message" for
// error elements and the quoted text of nodes without children. psi keeps
// KDoc as one token, so Kotlin's KDoc subtrees are cut off below KDOC and
// KDOC lines carry no text.

var update = flag.Bool("update", false, "rewrite psi/testdata/corpus.sum from -oracle dumps")
var oracle = flag.String("oracle", "", "directory of psidump --text --rel . dumps of testdata/**/*.kt")

// dump renders s as psidump --text does.
func dump(s *Snapshot) string {
	var sb strings.Builder
	depth := map[NodeID]int{}
	diags := s.Diagnostics()
	WalkNodes(s.RootNode(), func(n NodeRef) bool {
		d := 0
		if p := n.Parent(); p.Valid() {
			d = depth[p.ID()] + 1
		}
		depth[n.ID()] = d
		r := n.Range()
		a, _ := s.UTF16Offset(r.Start)
		b, _ := s.UTF16Offset(r.End)
		fmt.Fprintf(&sb, "%d %s %d %d", d, n.Kind(), a, b)
		if n.NumericKind() == KindERROR_ELEMENT {
			sb.WriteString(" !")
			msg := "?"
			if len(diags) > 0 {
				msg, diags = diags[0].Message, diags[1:]
			}
			if msg == starProjectionNotAType {
				msg = "Type expected" // psi's one deliberate message change
			}
			sb.WriteString(quoteDump(msg))
		}
		if !n.FirstChild().Valid() && n.NumericKind() != KindKDOC {
			sb.WriteByte(' ')
			sb.WriteString(quoteDump(n.Text()))
		}
		sb.WriteByte('\n')
		return true
	})
	return sb.String()
}

func quoteDump(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// collapseKDoc drops the lines below each KDOC line of an oracle dump and
// strips the KDOC line's text.
func collapseKDoc(dump string) string {
	var sb strings.Builder
	cut := -1
	for _, line := range strings.SplitAfter(dump, "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, " ", 3)
		d, _ := strconv.Atoi(f[0])
		if cut >= 0 && d > cut {
			continue
		}
		cut = -1
		if f[1] == "KDOC" {
			cut = d
		}
		sb.WriteString(line)
	}
	return sb.String()
}

func TestGoldenTrees(t *testing.T) {
	srcs, _ := filepath.Glob("testdata/src/*")
	if len(srcs) == 0 {
		t.Fatal("no fixtures")
	}
	for _, src := range srcs {
		name := filepath.Base(src)
		t.Run(name, func(t *testing.T) {
			text, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata/trees", name+".tree"))
			if err != nil {
				t.Fatal(err)
			}
			got := dump(Parse(name, string(text)))
			if w := collapseKDoc(string(want)); got != w {
				t.Errorf("tree mismatch:\n%s", firstDiff(got, w))
			}
		})
	}
}

// firstDiff shows the first differing line with context.
func firstDiff(got, want string) string {
	g := strings.Split(got, "\n")
	w := strings.Split(want, "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			var sb strings.Builder
			for j := max(0, i-5); j < i; j++ {
				fmt.Fprintf(&sb, "    %s\n", w[j])
			}
			fmt.Fprintf(&sb, "got  %s\nwant %s", gl, wl)
			return sb.String()
		}
	}
	return ""
}

// corpusFiles lists the repository's testdata/**/*.kt, relative to the
// repository root.
func corpusFiles(t testing.TB) []string {
	var out []string
	err := filepath.WalkDir("../testdata", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".kt") {
			out = append(out, strings.TrimPrefix(filepath.ToSlash(p), "../"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func readSums(t *testing.T) map[string]string {
	f, err := os.Open("testdata/corpus.sum")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sums := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if sum, name, ok := strings.Cut(sc.Text(), "  "); ok {
			sums[name] = sum
		}
	}
	return sums
}

func TestCorpusTrees(t *testing.T) {
	files := corpusFiles(t)
	if *update {
		if *oracle == "" {
			t.Fatal("-update needs -oracle")
		}
		var sb strings.Builder
		for _, f := range files {
			d, err := os.ReadFile(filepath.Join(*oracle, f+".tree"))
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&sb, "%x  %s\n", sha256.Sum256([]byte(collapseKDoc(string(d)))), f)
		}
		if err := os.WriteFile("testdata/corpus.sum", []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	sums := readSums(t)
	bad := 0
	for _, f := range files {
		text, err := os.ReadFile("../" + f)
		if err != nil {
			t.Fatal(err)
		}
		want, ok := sums[f]
		if !ok {
			t.Errorf("%s: no sum in testdata/corpus.sum; rerun with -update", f)
			continue
		}
		got := dump(Parse(f, string(text)))
		if fmt.Sprintf("%x", sha256.Sum256([]byte(got))) != want {
			bad++
			msg := "tree differs from stock Kotlin"
			if *oracle != "" {
				if d, err := os.ReadFile(filepath.Join(*oracle, f+".tree")); err == nil {
					msg += ":\n" + firstDiff(got, collapseKDoc(string(d)))
				}
			}
			t.Errorf("%s: %s", f, msg)
		}
	}
	if bad > 0 {
		t.Logf("%d of %d files differ", bad, len(files))
	}
}

func TestStarProjectionDiagnostic(t *testing.T) {
	const src = "@Composable fun Host(vm: * = viewModel()) {}"
	s := Parse("Case.kt", src)
	ds := s.Diagnostics()
	if len(ds) != 1 || ds[0].Code != "syntax_error" || ds[0].Message != starProjectionNotAType {
		t.Fatalf("diagnostics = %v", ds)
	}
	var typ NodeRef
	WalkNodes(s.RootNode(), func(n NodeRef) bool {
		if n.NumericKind() == KindVALUE_PARAMETER {
			typ = n.TypeReference()
		}
		return true
	})
	if typ.Text() != "*" || typ.Range() != ds[0].Range {
		t.Fatalf("type %q %v, diagnostic %v", typ.Text(), typ.Range(), ds[0].Range)
	}
	if s.RootNode().Text() != src {
		t.Fatal("not lossless")
	}
}
