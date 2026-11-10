package ide

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestInspections(t *testing.T) {
	cases := []struct {
		name, src string
		want      string // "CODE text" of each warning
		fixed     string // the text after the first warning's fix; "" for none
	}{
		{"safe call", `fun f(s: String) = s?.length
`, "UNNECESSARY_SAFE_CALL ?.", `fun f(s: String) = s.length
`},
		{"safe call nullable", `fun f(s: String?) = s?.length
`, "", ""},
		{"safe call smart cast", `fun f(s: String?) = if (s != null) s?.length else 0
`, "", ""},
		{"safe call platform", `fun f() = System.getProperty("a")?.length
`, "", ""},
		{"assertion", `fun f(s: String) = s!!.length
`, "UNNECESSARY_NOT_NULL_ASSERTION !!", `fun f(s: String) = s.length
`},
		{"assertion nullable", `fun f(s: String?) = s!!.length
`, "", ""},
		{"assertion generic", `fun <T> f(t: T) = t!!
`, "", ""},
		{"cast", `fun f(s: String) = (s as String).length
`, "USELESS_CAST as String", `fun f(s: String) = (s).length
`},
		{"cast widening", `fun f(s: String) = s as Any
`, "", ""},
		{"cast nullable", `fun f(s: String?) = s as String
`, "", ""},
		{"cast literal", `val x = 1 as Int
`, "", ""},
		{"cast list", `fun f(l: List<String>) = l as List<String>
`, "USELESS_CAST as List<String>", `fun f(l: List<String>) = l
`},
		{"can be val", `fun f(): Int {
    var x = 1
    return x + 1
}
`, "CAN_BE_VAL var", `fun f(): Int {
    val x = 1
    return x + 1
}
`},
		{"assigned var", `fun f(): Int {
    var x = 1
    x = 2
    var y = 0
    y += x
    var z = 0
    z++
    var w = 0
    run { w = 3 }
    return y + z + w
}
`, "", ""},
		{"lateinit and class var", `class A {
    var a = 1
    fun f(): Int {
        var b: Int
        b = 2
        return a + b
    }
}
`, "", ""},
		{"unused parameter", `private fun g(a: Int, b: Int) = a

fun h() = g(1, 2) + g(b = 3, a = 4)
`, "UNUSED_PARAMETER b", `private fun g(a: Int) = a

fun h() = g(1) + g(a = 4)
`},
		{"unused first parameter", `fun h(): Int {
    fun g(a: Int, b: Int): Int {
        return b
    }
    return g(1, 2)
}
`, "UNUSED_PARAMETER a", `fun h(): Int {
    fun g(b: Int): Int {
        return b
    }
    return g(2)
}
`},
		{"parameter by reference", `private fun g(a: Int) = 1

fun h() = listOf(1).map(::g)
`, "UNUSED_PARAMETER a", ""},
		{"public parameter", `fun g(a: Int) = 1
`, "", ""},
		{"parameter in template", "private fun g(a: Int) = \"$a\"\n\nfun h() = g(1)\n", "", ""},
		{"parameter in default", `private fun g(a: Int, b: Int = a) = b

fun h() = g(1)
`, "", ""},
		{"override parameter", `interface I {
    fun f(a: Int): Int
}

private class C : I {
    override fun f(a: Int) = 1
}

fun h(): I = C()
`, "", ""},
		{"main parameter", `fun main(args: Array<String>) {}
`, "", ""},
		{"unused private", `class A {
    private fun unused() = 1

    private val v = 2

    fun used() = v
}
`, "UNUSED_PRIVATE_MEMBER unused", `class A {

    private val v = 2

    fun used() = v
}
`},
		{"unused private property", `private val p = 1

fun f() = 2
`, "UNUSED_PRIVATE_MEMBER p", `
fun f() = 2
`},
		{"private overloads", `private fun o(a: Int) = a
private fun o(a: String) = a

fun f() = o(1)
`, "", ""},
		{"private operator", `class V {
    private operator fun plus(o: V) = this
    fun f() = this + this
}
`, "", ""},
		{"private reflected", `class A {
    private fun r() = 1
}

fun f() = A::class.java.getDeclaredMethod("r")
`, "", ""},
		{"semicolon", "fun f() {\n    val x = 1;\n    println(x); println(x)\n}\n", "REDUNDANT_SEMICOLON ;", "fun f() {\n    val x = 1\n    println(x); println(x)\n}\n"},
		{"semicolon before lambda", "fun g(f: () -> Unit) = f()\n\nfun f() {\n    val x = 1;\n    { println(x) }()\n}\n", "", ""},
		{"empty loop body", "fun f() {\n    var i = 0\n    while (++i < 3);\n    for (x in 1..2);\n    if (i > 0);\n    println(i)\n}\n", "", ""},
		{"do while semicolon", "fun f() {\n    var i = 0\n    do while (i++ < 5);\n    println(i)\n}\n", "REDUNDANT_SEMICOLON ;", "fun f() {\n    var i = 0\n    do while (i++ < 5)\n    println(i)\n}\n"},
		{"enum semicolon", "enum class E { A, B; }\n", "REDUNDANT_SEMICOLON ;", "enum class E { A, B }\n"},
		{"enum semicolon with members", "enum class E {\n    A, B;\n\n    fun f() = 1\n}\n", "", ""},
		{"unit return type", "fun f(): Unit {\n    println()\n}\n", "REDUNDANT_UNIT_RETURN_TYPE Unit", "fun f() {\n    println()\n}\n"},
		{"unit override", "interface I {\n    fun f()\n}\n\nclass A : I {\n    override fun f(): Unit {\n    }\n}\n", "", ""},
		{"unit expression body", "fun f(): Unit = println()\n", "", ""},
		{"public", "public class A {\n    public fun f() = 1\n}\n", "REDUNDANT_VISIBILITY_MODIFIER public; REDUNDANT_VISIBILITY_MODIFIER public", "class A {\n    public fun f() = 1\n}\n"},
		{"public override", "abstract class B {\n    protected abstract fun f(): Int\n}\n\nclass C : B() {\n    public override fun f() = 1\n}\n", "", ""},
		{"final", "class A {\n    final fun f() = 1\n}\n", "REDUNDANT_MODALITY_MODIFIER final", "class A {\n    fun f() = 1\n}\n"},
		{"final in open class", "open class A {\n    final fun f() = 1\n}\n", "", ""},
		{"interface modifiers", "interface I {\n    open fun f() = 1\n    abstract fun g(): Int\n}\n", "REDUNDANT_MODALITY_MODIFIER open; REDUNDANT_MODALITY_MODIFIER abstract", "interface I {\n    fun f() = 1\n    abstract fun g(): Int\n}\n"},
		{"empty class body", "class A {}\n\nobject O {\n}\n", "REMOVE_EMPTY_CLASS_BODY {}; REMOVE_EMPTY_CLASS_BODY {\n}", "class A\n\nobject O {\n}\n"},
		{"empty object literal body", "interface I\n\nfun f(): I = object : I {}\n", "", ""},
		{"enum without entries", "enum class E {;\n    fun f() = 1\n}\n", "", ""},
		{"lambda on next line", "fun <T> build(f: () -> T): T = f()\n\nfun f() {\n    val x = build()\n    { 1 }\n    println(x)\n}\n", "", ""},
		{"empty body comment", "class A {\n    // later\n}\n", "", ""},
		{"empty constructor", "class A()\n", "REMOVE_EMPTY_PRIMARY_CONSTRUCTOR ()", "class A\n"},
		{"empty constructor secondary", "class A() {\n    constructor(x: Int) : this()\n}\n", "", ""},
		{"constructor keyword", "class A constructor(val x: Int)\n", "REDUNDANT_CONSTRUCTOR_KEYWORD constructor", "class A(val x: Int)\n"},
		{"private constructor", "class A private constructor(val x: Int)\n", "", ""},
		{"lambda parentheses", "fun f() = listOf(1).map() { it + 1 }\n", "REMOVE_EMPTY_PARENTHESES_FROM_LAMBDA_CALL ()", "fun f() = listOf(1).map { it + 1 }\n"},
		{"template braces", "fun f(x: Int) = \"${x} ${x}a ${x + 1}\"\n\nclass A {\n    override fun toString() = \"${this.hashCode()}\"\n}\n", "REMOVE_CURLY_BRACES_FROM_TEMPLATE ${x}", "fun f(x: Int) = \"$x ${x}a ${x + 1}\"\n\nclass A {\n    override fun toString() = \"${this.hashCode()}\"\n}\n"},
		{"template this", "class A {\n    fun f() = \"${this}\"\n}\n", "REMOVE_CURLY_BRACES_FROM_TEMPLATE ${this}", "class A {\n    fun f() = \"$this\"\n}\n"},
		{"broken file", `fun f(s: String) = s?.length + undefined
`, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := psi.Parse("A.kt", tc.src)
			s := analysis.Open(stdlib(t))
			defer s.Close()
			res, err := s.Update(context.Background(), []*psi.Snapshot{snap})
			if err != nil {
				t.Fatal(err)
			}
			v := NewView(res, []*psi.Snapshot{snap})
			ds := Inspections(v, "A.kt")
			var got []string
			for _, d := range ds {
				got = append(got, d.Code+" "+tc.src[d.Range.Start:d.Range.End])
			}
			if g := strings.Join(got, "; "); g != tc.want {
				t.Fatalf("got %q, want %q", g, tc.want)
			}
			if len(ds) == 0 {
				return
			}
			doc := NewDocument("A.kt", 1, tc.src)
			fixes := QuickFixes(v, doc, ds[0], FixEnv{})
			if tc.fixed == "" {
				if len(fixes) > 0 {
					t.Fatalf("unexpected fix %q", fixes[0].Title)
				}
				return
			}
			if len(fixes) != 1 {
				t.Fatalf("got %d fixes", len(fixes))
			}
			after := applyScanEdits(tc.src, fixes[0].Edits)
			if after != tc.fixed {
				t.Fatalf("%s:\n%s\nwant:\n%s", fixes[0].Title, after, tc.fixed)
			}
			res, err = s.Update(context.Background(), []*psi.Snapshot{psi.Parse("A.kt", after)})
			if err != nil {
				t.Fatal(err)
			}
			if ds := res.Diagnostics(); len(ds) > 0 {
				t.Fatalf("diagnostics after the fix: %v", ds)
			}
		})
	}
}

// TestInspectionsScan applies the fix of every inspection's warning in
// the programs of testdata/run, one at a time. Each result must still
// check without errors. The ones known to break a program are listed in
// testdata/inspect-budget.txt; the list may only shrink.
func TestInspectionsScan(t *testing.T) {
	budget := map[string]bool{}
	if b, err := os.ReadFile("testdata/inspect-budget.txt"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				budget[l] = true
			}
		}
	}
	failing := map[string]bool{}
	cfg := stdlib(t)
	if _, err := classpath.FindJDK(); err != nil {
		t.Skipf("no JDK (set JAVA_HOME): %v", err)
	}
	cfg.NoJDK = false
	files, _ := filepath.Glob("../../testdata/run/*.kt")
	dirs, _ := filepath.Glob("../../testdata/run/*/main.kt")
	mods := map[string][]string{}
	for _, f := range files {
		mods[strings.TrimSuffix(filepath.Base(f), ".kt")] = []string{f}
	}
	for _, d := range dirs {
		kts, _ := filepath.Glob(filepath.Join(filepath.Dir(d), "*.kt"))
		mods[filepath.Base(filepath.Dir(d))] = kts
	}
	names := slices.Sorted(mapKeys(mods))
	s := analysis.Open(cfg)
	defer s.Close()
	check := analysis.Open(cfg)
	defer check.Close()
	found, applied := map[string]int{}, map[string]int{}
	var failed int
	for k, name := range names {
		if testing.Short() && k%10 != 0 {
			continue
		}
		var snaps []*psi.Snapshot
		for _, p := range mods[name] {
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			snaps = append(snaps, psi.Parse(filepath.Base(p), string(src)))
		}
		res, err := s.Update(context.Background(), snaps)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Diagnostics()) > 0 {
			continue
		}
		v := NewView(res, snaps)
		for _, sn := range snaps {
			doc := NewDocument(sn.Path(), 1, sn.Source())
			for _, d := range Inspections(v, sn.Path()) {
				found[d.Code]++
				for _, fx := range QuickFixes(v, doc, d, FixEnv{}) {
					applied[d.Code]++
					var after []*psi.Snapshot
					for _, o := range snaps {
						src := o.Source()
						if o == sn {
							src = applyScanEdits(src, fx.Edits)
						}
						after = append(after, psi.Parse(o.Path(), src))
					}
					res, err := check.Update(context.Background(), after)
					if err != nil {
						t.Fatal(err)
					}
					line, col, _ := sn.Position(d.Range.Start)
					id := fmt.Sprintf("%s %s:%d:%d %s", name, sn.Path(), line, col, d.Code)
					if os.Getenv("KC_INSPECT_LOG") != "" {
						t.Logf("%s %q", id, sn.Source()[d.Range.Start:d.Range.End])
					}
					if findingCodes[d.Code] {
						// The fix removes the finding, and makes no new one.
						count := func(snap *psi.Snapshot) int {
							n := 0
							for _, x := range styleFindings(snap, "") {
								if x.Code == d.Code {
									n++
								}
							}
							return n
						}
						for _, a := range after {
							if a.Path() == sn.Path() && count(a) != count(sn)-1 {
								t.Errorf("%s: %d findings after the fix, %d before", id, count(a), count(sn))
							}
						}
					}
					if ds := res.Diagnostics(); len(ds) > 0 {
						failed++
						failing[id] = true
						if budget[id] {
							continue
						}
						var msgs []string
						for _, e := range ds[:min(3, len(ds))] {
							msgs = append(msgs, fmt.Sprintf("%s:%d:%d %s", e.Path, e.Line, e.Column, e.Message))
						}
						t.Errorf("%s: %s", id, strings.Join(msgs, "; "))
					} else if budget[id] && !testing.Short() {
						t.Errorf("%s now applies cleanly: remove it from testdata/inspect-budget.txt", id)
					}
				}
			}
		}
	}
	t.Logf("found %v, applied %v, %d failed (%d budgeted)", found, applied, failed, len(budget))
	if os.Getenv("KC_INSPECT_BUDGET") != "" {
		ids := slices.Sorted(mapKeys(failing))
		os.WriteFile("testdata/inspect-budget.txt", []byte("# Inspection fixes TestInspectionsScan knows to break a program: module path:line:col code.\n"+strings.Join(ids, "\n")+"\n"), 0o644)
	}
}
