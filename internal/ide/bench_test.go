package ide

import (
	"os"
	"testing"
)

// BenchmarkKeystroke measures the syntax tier's work per keystroke: apply a
// one-character edit in the middle of the file, reparse, and compute syntax
// diagnostics. KC_LSP_BENCH
// names the file; the default is the repo's largest Kotlin source.
func BenchmarkKeystroke(b *testing.B) {
	path := os.Getenv("KC_LSP_BENCH")
	if path == "" {
		path = "../../jvm/kc-aa/src/io/github/barteo/kc/aa/KcRound.kt"
	}
	src, err := os.ReadFile(path)
	if err != nil {
		b.Skip(err)
	}
	d := NewDocument(path, 1, string(src))
	mid := d.Pos(len(src)/2, UTF16)
	b.SetBytes(int64(len(src)))
	spliced := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text := " "
		if i%2 == 1 {
			text = ""
		}
		to := mid
		if i%2 == 1 {
			to.Char++
		}
		d, err = d.Apply(i+2, []Edit{{From: mid, To: to, Text: text}}, UTF16)
		if err != nil {
			b.Fatal(err)
		}
		_ = SyntaxDiagnostics(d)
		if d.Snap.Reparsed() {
			spliced++
		}
	}
	b.ReportMetric(float64(spliced)/float64(b.N), "spliced/op")
}
