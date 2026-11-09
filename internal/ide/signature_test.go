package ide

import (
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis"
)

func TestSignatureAt(t *testing.T) {
	P := analysis.ProbeName
	for _, tc := range []struct {
		src   string
		ok    bool
		arg   int
		name  string
		probe string // the call in ProbeText
	}{
		{"f(|)", true, 0, "", "f( " + P + " )"},
		{"f(1, |)", true, 1, "", "f(1,  " + P + " )"},
		{"f(1, ab|c)", true, 1, "", "f(1,  " + P + " )"},
		{"f(1, 2|)", true, 1, "", "f(1,  " + P + " )"},
		{"f(x = |)", true, 0, "x", "f(x = " + P},
		{"f(x = 1, y = g|)", true, 1, "y", "f(x = 1, y = " + P + " )"},
		{"f(g(1, |))", true, 1, "", "g(1,  " + P + " )"},
		{"f(1, |\n", true, 1, "", "f(1,  " + P + ") "},
		{"f(1)|", false, 0, "", ""},
		{"f { |}", false, 0, "", ""},
		{"f(\"a|\")", false, 0, "", ""},
	} {
		src := "fun t() {\n    " + tc.src + "\n}\n"
		off := strings.Index(src, "|")
		sc, ok := SignatureAt(NewDocument("a.kt", 1, src[:off]+src[off+1:]), off)
		if ok != tc.ok {
			t.Errorf("%q: ok = %v", tc.src, ok)
			continue
		}
		if !ok {
			continue
		}
		if sc.Argument != tc.arg || sc.Name != tc.name || !strings.Contains(sc.ProbeText, tc.probe) {
			t.Errorf("%q: argument %d, name %q, probe text %q", tc.src, sc.Argument, sc.Name, sc.ProbeText)
		}
	}
}

func TestSignatureHelp(t *testing.T) {
	str, i := analysis.Type{Name: "kotlin.String"}, analysis.Type{Name: "kotlin.Int"}
	c := &analysis.Completion{Calls: []analysis.CompletionCall{{Argument: 1, Candidates: []analysis.Call{
		{Name: "p.f", Parameters: []analysis.Type{str, i, i}, ParameterNames: []string{"a", "b", "c"}, Varargs: make([]bool, 3)},
		{Name: "p.f", Parameters: []analysis.Type{str}, ParameterNames: []string{"a"}, Varargs: make([]bool, 1)},
	}}}}
	sigs, active := SignatureHelp(SignatureContext{}, c)
	if len(sigs) != 2 || active != 1 || sigs[0].Label != "fun f(a: String)" || sigs[0].Active != -1 || sigs[1].Active != 1 {
		t.Fatalf("%+v, active %d", sigs, active)
	}
	if pr := sigs[1].Parameters[1]; sigs[1].Label[pr[0]:pr[1]] != "b: Int" {
		t.Errorf("parameter b at %v of %q", pr, sigs[1].Label)
	}
}
