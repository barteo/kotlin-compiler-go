package lsp

import (
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestShiftDiagnostics(t *testing.T) {
	r := func(start, end int) psi.Range { return psi.Range{Start: start, End: end} }
	d := func(start, end int) ide.Diagnostic { return ide.Diagnostic{Range: r(start, end)} }
	//       0123456789
	a := "abc def ghi"
	ds := []ide.Diagnostic{d(0, 3), d(4, 7), d(8, 11), d(3, 3)}
	for _, tc := range []struct {
		b    string
		want []psi.Range
	}{
		{a, []psi.Range{r(0, 3), r(4, 7), r(8, 11), r(3, 3)}},
		// An insertion before def moves it and what follows.
		{"abc XX def ghi", []psi.Range{r(0, 3), r(7, 10), r(11, 14), r(3, 3)}},
		// A change inside def drops it.
		{"abc dxf ghi", []psi.Range{r(0, 3), r(8, 11), r(3, 3)}},
		// Deleting def drops it and moves ghi back.
		{"abc  ghi", []psi.Range{r(0, 3), r(5, 8), r(3, 3)}},
		// Appending keeps everything.
		{a + "!", []psi.Range{r(0, 3), r(4, 7), r(8, 11), r(3, 3)}},
	} {
		var got []psi.Range
		for _, d := range shiftDiagnostics(ds, a, tc.b) {
			got = append(got, d.Range)
		}
		if len(got) != len(tc.want) {
			t.Errorf("%q: %v, want %v", tc.b, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%q: %v, want %v", tc.b, got, tc.want)
				break
			}
		}
	}
}
