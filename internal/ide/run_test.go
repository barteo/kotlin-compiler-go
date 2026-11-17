package ide

import (
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestMainFunctions(t *testing.T) {
	for _, tc := range []struct {
		path, src string
		want      []string
	}{
		{"app/Main.kt", "package a.b\n\nfun main() {}\n", []string{"a.b.MainKt"}},
		{"fizz-buzz.kt", "fun main(args: Array<String>) = Unit\n", []string{"Fizz_buzzKt"}},
		{"a.kt", "@file:JvmName(\"Runner\")\npackage p\n\nsuspend fun main(vararg args: String) {}\n", []string{"p.Runner"}},
		{"a.kt", "package p\nobject O { @JvmStatic fun main(args: Array<String>) {} }\nclass C { companion object { @JvmStatic fun main(vararg args: String) {} } }\n",
			[]string{"p.O", "p.C"}},
		{"a.kt", "package p\nobject O { fun main(args: Array<String>) {} }\nobject P { @JvmStatic fun main() {} }\n", nil},
		{"a.kt", "fun main(x: Int) {}\nprivate fun main() {}\nfun String.main() {}\nfun <T> main() {}\nfun main(): Int = 1\nclass C { fun main() {} }\n", nil},
	} {
		var got []string
		for _, m := range MainFunctions(psi.Parse(tc.path, tc.src)) {
			got = append(got, m.Class)
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s: %v, want %v", tc.src, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: %v, want %v", tc.src, got, tc.want)
			}
		}
	}
}
