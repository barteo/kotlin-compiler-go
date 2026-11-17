package ide

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

const semSrc = `package p

import kotlin.math.abs

enum class Color { RED }
data class P(val a: Int, val b: Int)
interface Named { val name: String }

class Box<T>(val item: T, var count: Int) : Named {
    override val name = "box"
    fun twice(f: (T) -> Int): Int = f(item) * 2
}

fun use(xs: List<Int>, flag: Boolean) {
    val b = Box("s", 1)
    var n = b.twice { s -> s.length } + xs.size
    val c = Color.RED
    val (x, y) = P(1, 2)
    n += abs(-3) + kotlin.math.max(n, 4)
    b.count = maxOf(1, 2, 3)
    println(listOf(1, -2).sum() + b.twice(f = { 0 }) + x + y)
}
`

func TestReferenceTokens(t *testing.T) {
	v := testView(t, "a.kt", semSrc)
	var got []string
	for _, tok := range ReferenceTokens(v, "a.kt") {
		line, col, _ := v.Snapshot("a.kt").Position(tok.Range.Start)
		got = append(got, fmt.Sprintf("%d:%d %s %s %d", line, col, semSrc[tok.Range.Start:tok.Range.End], TokenTypeNames[tok.Type], tok.Mods))
	}
	slices.Sort(got)
	want := []string{
		"11:19 T typeParameter 0",
		"11:25 Int class 0",
		"11:31 Int class 0",
		"11:37 f parameter 0",
		"11:39 item property 2",
		"14:13 List class 0",
		"14:18 Int class 0",
		"14:30 Boolean class 0",
		"15:13 Box class 0",
		"16:13 b variable 2",
		"16:15 twice method 0",
		"16:28 s parameter 0",
		"16:30 length property 0",
		"16:41 xs parameter 0",
		"16:44 size property 2",
		"17:13 Color enum 0",
		"17:19 RED enumMember 2",
		"18:18 P class 0",
		"19:10 abs function 0",
		"19:20 kotlin namespace 0",
		"19:27 math namespace 0",
		"19:32 max function 0",
		"19:36 n variable 0",
		"19:5 n variable 0",
		"20:15 maxOf function 0",
		"20:5 b variable 2",
		"20:7 count property 0",
		"21:13 listOf function 0",
		"21:27 sum function 0",
		"21:35 b variable 2",
		"21:37 twice method 0",
		"21:5 println function 0",
		"21:56 x variable 0",
		"21:60 y variable 0",
		"6:21 Int class 0",
		"6:33 Int class 0",
		"7:29 String class 0",
		"9:24 T typeParameter 0",
		"9:38 Int class 0",
		"9:45 Named interface 0",
	}
	if !slices.Equal(got, want) {
		t.Errorf("tokens:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestInlayHints(t *testing.T) {
	v := testView(t, "a.kt", semSrc)
	var got []string
	for _, h := range InlayHints(v, "a.kt", psi.Range{Start: 0, End: len(semSrc)}) {
		line, col, _ := v.Snapshot("a.kt").Position(h.Offset)
		got = append(got, fmt.Sprintf("%d:%d %q %d", line, col, h.Label, h.Kind))
	}
	want := []string{
		"15:17 \"item:\" 2",
		"15:22 \"count:\" 2",
		"16:10 \": Int\" 1",
		"16:24 \": String\" 1",
		"17:10 \": Color\" 1",
		"18:11 \": Int\" 1",
		"18:14 \": Int\" 1",
		"18:20 \"a:\" 2",
		"18:23 \"b:\" 2",
		"19:14 \"n:\" 2",
		"19:39 \"b:\" 2",
		"20:21 \"a:\" 2",
		"20:24 \"b:\" 2",
		"20:27 \"c:\" 2",
		"21:20 \"elements:\" 2",
	}
	if !slices.Equal(got, want) {
		t.Errorf("hints:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// A range selects the hints in it.
	line17 := caret(t, semSrc, "|    val c")
	if got := InlayHints(v, "a.kt", psi.Range{Start: line17, End: line17 + 22}); len(got) != 1 || got[0].Label != ": Color" {
		t.Errorf("hints of one line: %v", got)
	}
}
