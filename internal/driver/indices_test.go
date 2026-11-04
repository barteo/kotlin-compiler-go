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
	lib := stdlib()
	if lib == "" {
		t.Skip("kotlin-stdlib.jar not found: set KOTLIN_HOME")
	}
	out := t.TempDir()
	opts := Options{OutDir: out, Jobs: runtime.NumCPU(), ClassPath: []string{lib}}
	if err := Compile([]string{"../../testdata/run/stdlibIndicesLoops.kt"}, opts); err != nil {
		t.Fatalf("compile: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "StdlibIndicesLoopsKt.class"))
	if err != nil {
		t.Fatal(err)
	}
	ms, err := classfile.ReadMethods(data)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, m := range ms {
		if !strings.HasPrefix(m.Name, "counted") || m.Code == nil {
			continue
		}
		n++
		insns, err := m.Decode()
		if err != nil {
			t.Fatalf("%s%s: %v", m.Name, m.Desc, err)
		}
		for _, in := range insns {
			if in.Op < classfile.INVOKEVIRTUAL || in.Op > classfile.INVOKEINTERFACE {
				continue
			}
			if strings.HasPrefix(in.Name, "getIndices") || in.Name == "iterator" || in.Name == "reversed" || in.Name == "step" ||
				strings.HasPrefix(in.Owner, "kotlin/ranges/") && in.Name != "checkStepIsPositive" || strings.HasSuffix(in.Owner, "Iterator") {
				t.Errorf("%s%s calls %s.%s%s", m.Name, m.Desc, in.Owner, in.Name, in.Desc)
			}
		}
	}
	if n < 30 {
		t.Errorf("found %d counted* methods, want at least 30", n)
	}
}
