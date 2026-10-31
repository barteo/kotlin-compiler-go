package inline

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
)

// The packages every Kotlin file imports, as in internal/check.
var defaultPackages = []string{
	"kotlin", "kotlin.annotation", "kotlin.collections", "kotlin.comparisons", "kotlin.io",
	"kotlin.ranges", "kotlin.sequences", "kotlin.text", "kotlin.jvm",
}

type stdlibFunc struct {
	name, class string
	sig         kmeta.JVMSig
	private     bool
	reified     bool
}

// inlineFunctions returns the inline functions of the default imports.
func inlineFunctions(t *testing.T, cp *classpath.Path) []stdlibFunc {
	t.Helper()
	parts := map[string][]kmeta.Part{}
	for _, name := range cp.Resources(".kotlin_module") {
		data, err := cp.Resource(name)
		if err != nil {
			t.Fatal(err)
		}
		m, err := kmeta.ReadModule(data)
		if err != nil {
			t.Fatal(err)
		}
		for pkg, ps := range m.Packages {
			parts[pkg] = append(parts[pkg], ps...)
		}
	}
	var out []stdlibFunc
	for _, pkg := range defaultPackages {
		for _, part := range parts[pkg] {
			info, err := cp.Load(part.Class)
			if err != nil || info.Kotlin == nil {
				t.Fatalf("%s: %v", part.Class, err)
			}
			meta, err := kmeta.ReadPackage(info.Kotlin)
			if err != nil {
				t.Fatalf("%s: %v", part.Class, err)
			}
			private := map[string]bool{}
			for _, m := range info.Methods {
				if m.Access&cf.AccPublic == 0 {
					private[m.Name+m.Desc] = true
				}
			}
			funcs, _, _ := meta.Names()
			for _, name := range funcs {
				fs, err := meta.Functions(name)
				if err != nil {
					t.Fatalf("%s.%s: %v", part.Class, name, err)
				}
				for _, f := range fs {
					if !f.IsInline() || f.Flags.Visibility() != kmeta.Public {
						continue
					}
					sf := stdlibFunc{name: name, class: part.Class, sig: f.JVM, private: private[f.JVM.Name+f.JVM.Desc]}
					for _, p := range f.TypeParams {
						sf.reified = sf.reified || p.Reified
					}
					out = append(out, sf)
				}
			}
		}
	}
	return out
}

// TestStdlibBodies analyzes the body of every inline function of the
// default imports. It reports how many can be inlined, and why the others
// can't. Errors other than an unsupported body fail the test.
func TestStdlibBodies(t *testing.T) {
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open("", []string{jar})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	funcs := inlineFunctions(t, cp)
	classes := map[string][]byte{}
	var total, private, reified, ok, okPrivate, okReified, lambdas, inlinable int
	why := map[string][]string{}
	for _, f := range funcs {
		total++
		if f.private {
			private++
		}
		if f.reified {
			reified++
		}
		data := classes[f.class]
		if data == nil {
			if data, err = cp.Bytes(f.class); err != nil {
				t.Fatal(err)
			}
			classes[f.class] = data
		}
		read := func(name, desc string) (*cf.Method, error) { return cf.ReadMethod(data, name, desc) }
		m, err := read(f.sig.Name, f.sig.Desc)
		if err != nil {
			t.Fatalf("%s.%s%s: %v", f.class, f.sig.Name, f.sig.Desc, err)
		}
		b, err := Analyze(m, read)
		if _, unsupported := errors.AsType[*Error](err); unsupported {
			why[err.Error()] = append(why[err.Error()], f.name)
			continue
		}
		if err != nil {
			t.Fatalf("%s.%s%s: %v", f.class, f.sig.Name, f.sig.Desc, err)
		}
		ok++
		if f.private {
			okPrivate++
		}
		if f.reified {
			okReified++
		}
		for _, p := range b.Params {
			if p.Arity >= 0 {
				lambdas++
				if p.Inlinable {
					inlinable++
				}
			}
		}
	}
	t.Logf("%d inline functions in the default imports, %d of them inline-only, %d reified", total, private, reified)
	t.Logf("%d bodies can be inlined (%d inline-only, %d reified); %d of their %d function-typed parameters are inlinable",
		ok, okPrivate, okReified, inlinable, lambdas)
	var reasons []string
	for w, names := range why {
		reasons = append(reasons, fmt.Sprintf("%d: %s (%s)", len(names), w, strings.Join(first(names, 5), ", ")))
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		t.Log(r)
	}
	if ok < total*9/10 {
		t.Errorf("only %d of %d bodies can be inlined", ok, total)
	}
}

func first(ss []string, n int) []string {
	sort.Strings(ss)
	if len(ss) > n {
		return append(ss[:n:n], "...")
	}
	return ss
}

// TestAnalyze checks the analysis of a few bodies.
func TestAnalyze(t *testing.T) {
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	cp, err := classpath.Open("", []string{jar})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	body := func(class, name, desc string) *Body {
		t.Helper()
		data, err := cp.Bytes(class)
		if err != nil {
			t.Fatal(err)
		}
		read := func(name, desc string) (*cf.Method, error) { return cf.ReadMethod(data, name, desc) }
		m, err := read(name, desc)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Analyze(m, read)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return b
	}

	let := body("kotlin/StandardKt__StandardKt", "let", "(Ljava/lang/Object;Lkotlin/jvm/functions/Function1;)Ljava/lang/Object;")
	if p := let.Params[1]; !p.Inlinable || p.Arity != 1 || len(p.Loads) != 1 || len(p.Invokes) != 1 || p.InTry[0] {
		t.Errorf("let's block: %+v", p)
	}
	if p := let.Params[0]; p.Inlinable || p.Arity != -1 {
		t.Errorf("let's receiver: %+v", p)
	}
	if let.Owner(let.Class) != "kotlin/StandardKt" {
		t.Errorf("owner %s", let.Owner(let.Class))
	}

	use := body("kotlin/io/CloseableKt", "use", "(Ljava/io/Closeable;Lkotlin/jvm/functions/Function1;)Ljava/lang/Object;")
	if p := use.Params[1]; !p.Inlinable || len(p.Invokes) != 1 || p.InTry[0] {
		t.Errorf("use's block: %+v", p)
	}
	// A jump out of the block runs the finally block: closeFinally.
	if fs, ok := use.Finallies(use.Insns[use.Params[1].Invokes[0]].Off); !ok || len(fs) != 1 {
		t.Errorf("use's finally blocks: %v %v", fs, ok)
	} else if f := fs[0]; !slices.ContainsFunc(use.Insns[f.Start:f.End], func(in cf.Insn) bool { return in.Name == "closeFinally" }) {
		t.Errorf("use's finally block doesn't call closeFinally: %v", use.Insns[f.Start:f.End])
	}
	var markers int
	for _, d := range use.Drop {
		if d {
			markers++
		}
	}
	if markers != 11 {
		t.Errorf("%d instructions dropped from use, want 11: 4 markers and their constants, and a null check", markers)
	}

	// thenBy captures its selector in an object of a template class.
	thenBy := body("kotlin/comparisons/ComparisonsKt__ComparisonsKt", "thenBy",
		"(Ljava/util/Comparator;Lkotlin/jvm/functions/Function1;)Ljava/util/Comparator;")
	if p := thenBy.Params[1]; p.Inlinable || p.Why == "" {
		t.Errorf("thenBy's selector: %+v", p)
	}
}
