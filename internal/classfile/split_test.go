package classfile

import (
	"strings"
	"testing"
)

func TestSplitString(t *testing.T) {
	for _, s := range []string{
		"",
		"short",
		strings.Repeat("a", MaxStringConstant),
		strings.Repeat("a", MaxStringConstant+1),
		strings.Repeat("ż", 40000),
		strings.Repeat("a\x00", 30000),
		strings.Repeat("😀", 20000),
		"a" + strings.Repeat("😀", 20000),
	} {
		if got := ModifiedUTF8Len(s); got != len(modifiedUTF8(s)) {
			t.Errorf("ModifiedUTF8Len of %d bytes = %d, want %d", len(s), got, len(modifiedUTF8(s)))
		}
		parts := SplitString(s)
		if strings.Join(parts, "") != s {
			t.Errorf("SplitString of %d bytes: parts don't join to it", len(s))
		}
		for _, p := range parts {
			if n := ModifiedUTF8Len(p); n > MaxStringConstant {
				t.Errorf("SplitString of %d bytes: a part of %d bytes", len(s), n)
			}
			var pool Pool
			pool.String(p) // doesn't panic
		}
		if fits := ModifiedUTF8Len(s) <= MaxStringConstant; fits != (len(parts) == 1) {
			t.Errorf("SplitString of %d bytes: %d parts", len(s), len(parts))
		}
	}
}
