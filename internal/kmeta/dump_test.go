package kmeta

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
)

// TestDumpRaw prints the raw protobuf fields of the metadata of the class
// files named by $KMETA_DUMP (separated by commas), for comparing kc's
// metadata with kotlinc's while developing. It is skipped without it.
func TestDumpRaw(t *testing.T) {
	paths := os.Getenv("KMETA_DUMP")
	if paths == "" {
		t.Skip("set KMETA_DUMP to class files to dump")
	}
	for _, path := range strings.Split(paths, ",") {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := classfile.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		m := info.Kotlin
		if m == nil {
			fmt.Printf("%s: no metadata\n", path)
			continue
		}
		fmt.Printf("%s: k=%d mv=%v xi=%d d2=%q\n", path, m.K, m.MV, m.XI, m.D2)
		if len(m.D1) == 0 {
			continue
		}
		b := Bytes(m.D1)
		d := decoder{b}
		stt := d.bytes()
		fmt.Printf("  string table:\n")
		dumpRaw(stt, "    ")
		fmt.Printf("  message:\n")
		dumpRaw(d.b, "    ")
	}
}

// dumpRaw prints a message's fields, recursing into length-delimited ones
// that parse as messages.
func dumpRaw(msg []byte, indent string) {
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		switch w {
		case wireVarint:
			fmt.Printf("%s%d: %d\n", indent, f, d.varint())
		case wireBytes:
			b := d.bytes()
			if isMessage(b) && len(b) > 0 {
				fmt.Printf("%s%d: {\n", indent, f)
				dumpRaw(b, indent+"  ")
				fmt.Printf("%s}\n", indent)
			} else {
				fmt.Printf("%s%d: %q\n", indent, f, b)
			}
		default:
			fmt.Printf("%s%d: wire %d\n", indent, f, w)
			d.skip(w)
		}
	}
}

// isMessage reports whether b parses as a sequence of protobuf fields.
func isMessage(b []byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	d := decoder{b}
	for d.more() {
		f, w := d.key()
		if f == 0 || w != wireVarint && w != wireBytes {
			return false
		}
		d.skip(w)
	}
	return true
}
