package ide

import (
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestKDocEnter(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string // | is the caret after the Enter
	}{
		{"open", "class A {\n    /**\n    |\n    fun f(a: Int, b: String): Int = a\n}\n",
			"class A {\n    /**\n     * \n     * @param a\n     * @param b\n     * @return\n     */\n    fun f(a: Int, b: String): Int = a\n}\n"},
		{"open at end", "/**\n|", "/**\n * \n */"},
		{"open before a documented one", "/**\n|\nfun f() {}\n\n/** Doc. */\nfun g() {}\n",
			"/**\n * \n */\nfun f() {}\n\n/** Doc. */\nfun g() {}\n"},
		{"closed by the editor", "/**\n| */\nfun f(x: Int) {}\n", "/**\n * \n * @param x\n */\nfun f(x: Int) {}\n"},
		{"unit result", "/**\n|\nfun f(): Unit {}\n", "/**\n * \n */\nfun f(): Unit {}\n"},
		{"class", "/**\n|\nclass C(val x: Int)\n", "/**\n * \n */\nclass C(val x: Int)\n"},
		{"line", "/**\n * Adds.\n|\n */\nfun f() {}\n", "/**\n * Adds.\n * \n */\nfun f() {}\n"},
		{"first line with text", "    /** Adds.\n    |\n     */\n", "    /** Adds.\n     * \n     */\n"},
		{"existing text after", "/**\n|\n * Adds.\n */\nfun f(a: Int) {}\n", "/**\n * \n * Adds.\n */\nfun f(a: Int) {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			off := strings.Index(tc.src, "|")
			src := tc.src[:off] + tc.src[off+1:]
			es := OnType(psi.Parse("a.kt", src), off, "\n", FormatOptions{})
			if got := applyEdits(src, es); got != tc.want {
				t.Errorf("got\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
	// Outside a KDoc, an Enter only indents.
	src := "// c\n\nfun f() {}\n"
	if es, ok := kdocEnter(psi.Parse("a.kt", src), 5); ok {
		t.Errorf("after a line comment: %v", es)
	}
}
