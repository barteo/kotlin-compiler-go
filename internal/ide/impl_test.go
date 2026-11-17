package ide

import (
	"fmt"
	"slices"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

const implSrc = `package p

interface Shape { fun area(): Double }

abstract class Base : Shape {
    override fun area() = 0.0
}

open class Square(val s: Double) : Base() {
    override fun area() = s * s
}

class Tile : Square(1.0) {
    override fun area() = 2.0
}

object Unit1 : Shape { override fun area() = 1.0 }

fun findByName(arrayBuffer: Int) = arrayBuffer
`

func TestImplementations(t *testing.T) {
	v := testView(t, "a.kt", implSrc)
	for _, c := range []struct {
		at   string
		want []string
	}{
		{"interface S|hape", []string{"a.kt:5:16 Base", "a.kt:9:12 Square", "a.kt:13:7 Tile", "a.kt:17:8 Unit1"}},
		{"open class Squ|are", []string{"a.kt:13:7 Tile"}},
		{"fun a|rea(): Double", []string{"a.kt:6:18 area", "a.kt:10:18 area", "a.kt:14:18 area", "a.kt:17:37 area"}},
		{"override fun area() = s|", nil},
		{"Tile : Squ|are", []string{"a.kt:13:7 Tile"}},
	} {
		if got := locs(Implementations([]*View{v}, "a.kt", caret(t, implSrc, c.at))); !slices.Equal(got, c.want) {
			t.Errorf("implementations at %q: %q, want %q", c.at, got, c.want)
		}
	}
}

func TestWorkspaceSymbols(t *testing.T) {
	d := DocumentOf(psi.Parse("a.kt", implSrc), 0)
	for _, c := range []struct {
		q    string
		want []string
	}{
		{"area", []string{"area Shape", "area Base", "area Square", "area Tile", "area Unit1"}},
		{"sq", []string{"Square p"}},
		{"fBN", []string{"findByName p"}},
		{"Ut1", []string{"Unit1 p"}},
		{"zz", nil},
	} {
		var got []string
		for _, m := range WorkspaceSymbols([]*Document{d}, c.q, 256) {
			got = append(got, fmt.Sprintf("%s %s", m.Symbol.Name, m.Container))
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("query %q: %q, want %q", c.q, got, c.want)
		}
	}
	if n := len(WorkspaceSymbols([]*Document{d}, "", 3)); n != 3 {
		t.Errorf("limit 3 gave %d", n)
	}
	for _, c := range []struct {
		name, q string
		ok      bool
	}{
		{"ArrayBuffer", "AB", true}, {"ArrayBuffer", "ArBu", true}, {"ArrayBuffer", "AyB", false},
		{"MAX_SIZE", "ms", true}, {"findByName", "fbn", true}, {"findByName", "byn", false},
	} {
		if got := humps(c.name, c.q); got != c.ok {
			t.Errorf("humps(%q, %q) = %v", c.name, c.q, got)
		}
	}
}
