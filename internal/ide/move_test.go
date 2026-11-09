package ide

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestMoveEdits(t *testing.T) {
	files := []string{
		"a/A.kt", `package a

import c.k

class A {
    fun b() = B()
}

fun helper() = k() + local()

fun Int.twice() = this * 2
`,
		"a/B.kt", `package a

class B {
    fun a() = A()
    fun h() = helper() + 1.twice()
}

fun local() = 2
`,
		"d/D.kt", `package d

import a.A
import a.*

fun g() = A().hashCode() + helper() + a.helper() + B().h()
`,
		"c/C.kt", `package c

fun k() = 1
`,
	}
	cases := []struct {
		name  string
		moves []Move
		want  map[string]string // files' expected texts, of those that change
	}{
		{"file", []Move{{"a/A.kt", "c"}}, map[string]string{
			"a/A.kt": `package c

import a.B
import a.local

class A {
    fun b() = B()
}

fun helper() = k() + local()

fun Int.twice() = this * 2
`,
			"a/B.kt": `package a

import c.A
import c.helper
import c.twice

class B {
    fun a() = A()
    fun h() = helper() + 1.twice()
}

fun local() = 2
`,
			"d/D.kt": `package d

import c.A
import a.*
import c.helper

fun g() = A().hashCode() + helper() + c.helper() + B().h()
`,
		}},
		{"same package", []Move{{"a/A.kt", "a"}}, map[string]string{}},
		{"both", []Move{{"a/A.kt", "e"}, {"a/B.kt", "e"}}, nil},
		{"default package", []Move{{"c/C.kt", ""}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := testView(t, files...)
			edits := MoveEdits([]*View{v}, tc.moves)
			byPath := map[string][]FixEdit{}
			for _, e := range edits {
				byPath[e.Snap.Path()] = append(byPath[e.Snap.Path()], e)
			}
			var after []*psi.Snapshot
			got := map[string]string{}
			for i := 0; i < len(files); i += 2 {
				src := files[i+1]
				if es := byPath[files[i]]; len(es) > 0 {
					src = applyScanEdits(src, es)
					got[files[i]] = src
				}
				after = append(after, psi.Parse(files[i], src))
			}
			if tc.want != nil {
				for _, p := range slices.Sorted(mapKeys(got)) {
					if got[p] != tc.want[p] {
						t.Errorf("%s:\n%s\nwant:\n%s", p, got[p], tc.want[p])
					}
				}
				for p := range tc.want {
					if _, ok := got[p]; !ok {
						t.Errorf("%s not edited", p)
					}
				}
			}
			s := analysis.Open(stdlib(t))
			defer s.Close()
			res, err := s.Update(context.Background(), after)
			if err != nil {
				t.Fatal(err)
			}
			if ds := res.Diagnostics(); len(ds) > 0 {
				var b strings.Builder
				for _, p := range slices.Sorted(mapKeys(got)) {
					b.WriteString("-- " + p + " --\n" + got[p])
				}
				t.Errorf("diagnostics after the move: %v\n%s", ds, b.String())
			}
		})
	}
}
