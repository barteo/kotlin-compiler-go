package ide

import (
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

// TestRewrites applies the rewrite intentions as TestRefactors does.
func TestRewrites(t *testing.T) {
	cfg := stdlib(t)
	if _, err := classpath.FindJDK(); err != nil {
		t.Skipf("no JDK (set JAVA_HOME): %v", err)
	}
	jar := cfg.ClassPath[0]
	for _, tc := range []struct {
		name, title, src, want string
	}{
		{"if to when", "Replace 'if' with 'when'", `package p

fun f(x: Int): String {
    /*|*/if (x < 0) {
        return "neg"
    } else if (x == 0) return "zero"
    else {
        println(x)
    }
    return "pos"
}
`, `package p

fun f(x: Int): String {
    when {
        x < 0 -> {
            return "neg"
        }
        x == 0 -> return "zero"
        else -> {
            println(x)
        }
    }
    return "pos"
}
`},
		{"if to when, expression", "Replace 'if' with 'when'", `package p

fun f(x: Int) = /*|*/if (x < 0) "neg" else if (x == 0) "zero" else "pos"
`, `package p

fun f(x: Int) = when {
    x < 0 -> "neg"
    x == 0 -> "zero"
    else -> "pos"
}
`},
		{"if to when, not an else if", "Replace 'if' with 'when'", `package p

fun f(x: Int) = if (x < 0) "neg" else /*|*/if (x == 0) "zero" else "pos"
`, ""},
		{"when to if", "Replace 'when' with 'if'", `package p

fun f(x: Int): String {
    return /*|*/when {
        x < 0 -> "neg"
        x == 0, x == 1 -> {
            println(x)
            "small"
        }
        else -> "pos"
    }
}
`, `package p

fun f(x: Int): String {
    return if (x < 0) "neg"
    else if ((x == 0) || (x == 1)) {
        println(x)
        "small"
    }
    else "pos"
}
`},
		{"when to if, dangling else", "Replace 'when' with 'if'", `package p

fun f(a: Boolean, b: Boolean) {
    /*|*/when {
        a -> if (b) println(1)
        else -> println(2)
    }
}
`, ""},
		{"when to if, subject", "Replace 'when' with 'if'", `package p

fun f(x: Int) = /*|*/when (x) {
    0 -> "zero"
    else -> "other"
}
`, ""},
		{"add braces", "Add braces to 'if' statement", `package p

fun f(x: Int) {
    /*|*/if (x > 0) println(x)
}
`, `package p

fun f(x: Int) {
    if (x > 0) {
        println(x)
    }
}
`},
		{"add braces to else", "Add braces to 'else' statement", `package p

fun f(x: Int) {
    if (x > 0) println(x) /*|*/else println(-x)
}
`, `package p

fun f(x: Int) {
    if (x > 0) println(x) else {
        println(-x)
    }
}
`},
		{"add braces to for", "Add braces to 'for' statement", `package p

fun f() {
    /*|*/for (i in 1..3) println(i)
}
`, `package p

fun f() {
    for (i in 1..3) {
        println(i)
    }
}
`},
		{"remove braces", "Remove braces from 'while' statement", `package p

fun f() {
    var i = 0
    /*|*/while (i < 3) {
        i++
    }
}
`, `package p

fun f() {
    var i = 0
    while (i < 3) i++
}
`},
		{"remove braces, dangling else", "Remove braces from 'if' statement", `package p

fun f(a: Boolean, b: Boolean) {
    /*|*/if (a) {
        if (b) println(1)
    } else println(2)
}
`, ""},
		{"remove braces, declaration", "Remove braces from 'if' statement", `package p

fun f(a: Boolean) {
    /*|*/if (a) {
        val x = 1
    }
}
`, ""},
		{"invert if", "Invert 'if' condition", `package p

fun f(x: Int) = /*|*/if (x < 3) "small" else "big"
`, `package p

fun f(x: Int) = if (x >= 3) "big" else "small"
`},
		{"invert if, call", "Invert 'if' condition", `package p

fun f(s: String) {
    /*|*/if (s.startsWith("a")) {
        println(1)
    } else {
        println(2)
    }
}
`, `package p

fun f(s: String) {
    if (!s.startsWith("a")) {
        println(2)
    } else {
        println(1)
    }
}
`},
		{"invert if, no else", "Invert 'if' condition", `package p

fun f(x: Int) {
    /*|*/if (x < 3) println(x)
}
`, ""},
		{"concatenation to template", "Convert concatenation to template", `package p

fun f(name: String, n: Int) = "Hi, " + /*|*/name + "! " + n + "x" + (n + 1) + name.length
`, `package p

fun f(name: String, n: Int) = "Hi, $name! ${n}x${n + 1}${name.length}"
`},
		{"concatenation to template, a name first", "Convert concatenation to template", `package p

fun f(name: String) = name /*|*/+ "!"
`, `package p

fun f(name: String) = "$name!"
`},
		{"concatenation to template, not a String", "Convert concatenation to template", `package p

fun f(n: Int) = n /*|*/+ 1
`, ""},
		{"concatenation to template, Int first", "Convert concatenation to template", `package p

fun f(l: List<String>) = l /*|*/+ "a"
`, ""},
		{"template to concatenation", "Convert template to concatenation", `package p

fun f(name: String, n: Int) = "/*|*/Hi, $name! ${n + 1}x".length
`, `package p

fun f(name: String, n: Int) = ("Hi, " + name + "! " + (n + 1) + "x").length
`},
		{"template to concatenation, expression first", "Convert template to concatenation", `package p

fun f(n: Int) = "/*|*/$n items"
`, `package p

fun f(n: Int) = n.toString() + " items"
`},
		{"it to parameter", "Replace 'it' with explicit parameter", `package p

fun f(l: List<List<Int>>): List<Int> = l.map { /*|*/it.size + it.count { it == 1 } }
`, `package p

fun f(l: List<List<Int>>): List<Int> = l.map { list -> list.size + list.count { it == 1 } }
`},
		{"parameter to it", "Replace explicit parameter 's' with 'it'", `package p

fun f(l: List<List<Int>>): List<Int> = l.map { /*|*/s -> s.size + s.indexOf(1) }
`, `package p

fun f(l: List<List<Int>>): List<Int> = l.map { it.size + it.indexOf(1) }
`},
		{"parameter to it, nested it", "Replace explicit parameter 's' with 'it'", `package p

fun f(l: List<List<Int>>): List<Int> = l.map { /*|*/s -> s.count { it == 1 } }
`, ""},
		{"parameter to it, typed", "Replace explicit parameter 's' with 'it'", `package p

fun f(l: List<List<Int>>): List<Int> = l.map { /*|*/s: List<Int> -> s.size }
`, ""},
		{"it to parameter, not a lambda's", "Replace 'it' with explicit parameter", `package p

fun f(): Int {
    val it = 1
    return /*|*/it
}
`, ""},
		{"when to if, exhaustive", "Replace 'when' with 'if'", `package p

fun f(b: Boolean) = /*|*/when {
    b -> 1
    !b -> 2
    else -> 3
}

enum class E { A, B }

fun g(e: E): Int {
    return when (e) {
        E.A -> 1
        E.B -> 2
    }
}
`, `package p

fun f(b: Boolean) = if (b) 1
else if (!b) 2
else 3

enum class E { A, B }

fun g(e: E): Int {
    return when (e) {
        E.A -> 1
        E.B -> 2
    }
}
`},
		{"arguments on separate lines, a comment", "Put arguments on separate lines", `package p

fun g(a: Int, b: Int) = a

val x = g(/*|*/1, /* b */ 2)
`, ""},
		{"template to concatenation, raw", "Convert template to concatenation", `package p

fun f(n: Int) = """/*|*/$n items"""
`, ""},
		{"parameter to it, a branch", "Replace explicit parameter 'n' with 'it'", `package p

fun f(b: Boolean): (Int) -> Int = if (b) { /*|*/n -> n + 1 } else { n -> n }
`, ""},
		{"join declaration, nullable var", "Join declaration and assignment", `package p

fun f(): Int {
    var /*|*/i: Int?
    i = 10
    return i + 1
}
`, ""},
		{"template to concatenation, const", "Convert template to concatenation", `package p

const val N = "n"
const val T = "/*|*/a $N"
`, ""},
		{"join declaration", "Join declaration and assignment", `package p

fun f(a: Int): Int {
    val /*|*/x: Int
    x = a * 2
    return x
}
`, `package p

fun f(a: Int): Int {
    val x: Int = a * 2
    return x
}
`},
		{"join declaration, not next", "Join declaration and assignment", `package p

fun f(a: Int): Int {
    val /*|*/x: Int
    println(a)
    x = a * 2
    return x
}
`, ""},
		{"arguments on separate lines", "Put arguments on separate lines", `package p

fun g(a: Int, b: String, c: Boolean) = a

fun f() {
    val x = g(/*|*/1, "b", true)
}
`, `package p

fun g(a: Int, b: String, c: Boolean) = a

fun f() {
    val x = g(
        1,
        "b",
        true
    )
}
`},
		{"parameters on one line", "Put parameters on one line", `package p

fun g(
    /*|*/a: Int,
    b: String,
) = a
`, `package p

fun g(a: Int, b: String) = a
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := applyRefactor(t, cfg, tc.title, tc.src)
			if tc.want == "" {
				if ok {
					t.Fatalf("offered %q:\n%s", tc.title, got)
				}
				return
			}
			if !ok {
				t.Fatalf("no refactoring %q: %s", tc.title, got)
			}
			if got != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", got, tc.want)
			}
			checkCompiles(t, cfg, jar, got)
		})
	}
}
