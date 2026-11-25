package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Each file has 200 Kotlin lines. The consumer reads an unsaved declaration
// from module zero in both the single-module and four-module workspaces.
func benchWorkspace(b *testing.B, count, nmodules int) (root, path, consumer string) {
	b.Helper()
	root = b.TempDir()
	p := project{Version: 1}
	for j := 0; j < nmodules; j++ {
		name := fmt.Sprintf("m%d", j)
		m := &module{Name: name, SourceRoots: []string{name}}
		if j > 0 {
			m.Dependencies = []string{fmt.Sprintf("m%d", j-1)}
		}
		p.Modules = append(p.Modules, m)
	}
	model, err := json.Marshal(p)
	if err != nil {
		b.Fatal(err)
	}
	put(b, filepath.Join(root, "kc-lsp.json"), string(model))
	for j := 0; j < count; j++ {
		name := fmt.Sprintf("m%d", j%nmodules)
		f := filepath.Join(root, name, fmt.Sprintf("Type%d.kt", j))
		var text strings.Builder
		fmt.Fprintf(&text, "package bench\nclass Type%d {\n", j)
		for k := 0; k < 49; k++ {
			fmt.Fprintf(&text, "    fun f%d(n: Int): Int {\n        val value = n + %d\n        return value\n    }\n", k, k)
		}
		text.WriteString("}\n")
		fmt.Fprintf(&text, "fun read%d(x: Type0) = x.f0(1)\n", j)
		put(b, f, text.String())
		if j == 0 {
			path = f
		}
		if j == count-1 {
			consumer = f
		}
	}
	return
}
func BenchmarkWorkspace(b *testing.B) {
	for _, test := range []struct {
		name           string
		files, modules int
	}{{"8k-lines", 40, 1}, {"40k-lines", 200, 4}} {
		b.Run(test.name, func(b *testing.B) {
			b.Setenv("PATH", "")
			b.Setenv("JAVA_HOME", "")
			b.Setenv("KOTLIN_HOME", "")
			root, path, consumer := benchWorkspace(b, test.files, test.modules)
			ctx := context.Background()
			b.Run("ColdIndex", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					w := workspace{roots: []string{root}}
					w.reload(context.Background())
					_, err := w.check(ctx, consumer)
					w.close()
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			w := workspace{roots: []string{root}}
			w.reload(context.Background())
			defer w.close()
			i, err := w.check(ctx, consumer)
			if err != nil {
				b.Fatal(err)
			}
			snap := w.docs[consumer].snap
			off := strings.LastIndex(snap.Source(), "f0(")
			b.Run("Hover", func(b *testing.B) {
				for b.Loop() {
					i.hover(snap.NodeAt(off))
				}
			})
			b.Run("Completion", func(b *testing.B) {
				for b.Loop() {
					i.completion(ctx, snap, off)
				}
			})
			b.Run("EditToDiagnostics", func(b *testing.B) {
				original := w.docs[path].snap.Source()
				edit := strings.Replace(original, "n + 0", "n + 1", 1)
				n := 0
				for b.Loop() {
					text := original
					if n%2 == 0 {
						text = edit
					}
					n++
					w.set(path, text, n)
					i, err := w.check(ctx, consumer)
					if err != nil {
						b.Fatal(err)
					}
					diagnostics(snap, i)
				}
			})
		})
	}
}
