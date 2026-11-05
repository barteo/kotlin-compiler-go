package driver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// TestIndicesLoops checks that loops over the standard library's indices,
// and their reversed() and step, are counted loops, as kotlinc's: the
// functions named counted* of testdata/run/stdlibIndicesLoops.kt, which
// TestRun runs, call no getIndices, reversed or step, and no iterator.
// A non-constant step's check calls checkStepIsPositive, if it fails.
func TestIndicesLoops(t *testing.T) {
	loopCalls(t, "stdlibIndicesLoops", "counted", func(in *classfile.Insn) bool {
		return strings.HasPrefix(in.Name, "getIndices") || in.Name == "iterator" || in.Name == "reversed" || in.Name == "step" ||
			strings.HasPrefix(in.Owner, "kotlin/ranges/") && in.Name != "checkStepIsPositive" || strings.HasSuffix(in.Owner, "Iterator")
	}, 30)
}

// TestReversedLoops checks that loops over reversed() of a stepped
// progression, and over a reversed progression with a step, are counted
// loops with the bounds computed in place, as kotlinc's: the functions
// named counted* of testdata/run/stdlibReversedLoops.kt call no reversed
// or step, nothing else of kotlin.ranges but checkStepIsPositive, and no
// iterator.
func TestReversedLoops(t *testing.T) {
	loopCalls(t, "stdlibReversedLoops", "counted", func(in *classfile.Insn) bool {
		return in.Name == "iterator" || in.Name == "reversed" || in.Name == "step" ||
			strings.HasPrefix(in.Owner, "kotlin/ranges/") && in.Name != "checkStepIsPositive" || strings.HasSuffix(in.Owner, "Iterator")
	}, 20)
}

// TestProgressionLoops checks that loops over values of the standard
// library's progression classes are counted loops over their first, last
// and step, as kotlinc's: the functions named looped* of
// testdata/run/stdlibProgressionLoops.kt call no iterator.
func TestProgressionLoops(t *testing.T) {
	loopCalls(t, "stdlibProgressionLoops", "looped", func(in *classfile.Insn) bool {
		return in.Name == "iterator" || in.Name == "hasNext" || strings.HasSuffix(in.Owner, "Iterator")
	}, 15)
}

// loopCalls compiles testdata/run/name.kt with the standard library and
// fails on each call in its file class's methods named prefix* that bad
// reports, and if there are fewer than min of those methods.
func loopCalls(t *testing.T, name, prefix string, bad func(*classfile.Insn) bool, min int) {
	t.Helper()
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	out := t.TempDir()
	opts := Options{OutDir: out, Jobs: runtime.NumCPU(), ClassPath: []string{lib}}
	if err := Compile([]string{"../../testdata/run/" + name + ".kt"}, opts); err != nil {
		t.Fatalf("compile: %v", err)
	}
	class := strings.ToUpper(name[:1]) + name[1:] + "Kt.class"
	data, err := os.ReadFile(filepath.Join(out, class))
	if err != nil {
		t.Fatal(err)
	}
	ms, err := classfile.ReadMethods(data)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, m := range ms {
		if !strings.HasPrefix(m.Name, prefix) || m.Code == nil {
			continue
		}
		n++
		insns, err := m.Decode()
		if err != nil {
			t.Fatalf("%s%s: %v", m.Name, m.Desc, err)
		}
		for i := range insns {
			in := &insns[i]
			if in.Op < classfile.INVOKEVIRTUAL || in.Op > classfile.INVOKEINTERFACE {
				continue
			}
			if bad(in) {
				t.Errorf("%s%s calls %s.%s%s", m.Name, m.Desc, in.Owner, in.Name, in.Desc)
			}
		}
	}
	if n < min {
		t.Errorf("found %d %s* methods, want at least %d", n, prefix, min)
	}
}
