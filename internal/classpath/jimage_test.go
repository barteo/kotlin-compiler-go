package classpath

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJImage writes a jimage with one resource per class into
// home/lib/modules. A class's compressors, innermost first, name the
// decompressors in its headers; none stores it as is.
func writeJImage(t *testing.T, home string, classes []testImageClass) {
	t.Helper()
	var strs bytes.Buffer
	strs.WriteByte(0)
	strOff := map[string]uint32{"": 0}
	str := func(s string) uint32 {
		if off, ok := strOff[s]; ok {
			return off
		}
		off := uint32(strs.Len())
		strs.WriteString(s)
		strs.WriteByte(0)
		strOff[s] = off
		return off
	}
	le := binary.LittleEndian

	var locs, resources bytes.Buffer
	var offsets []uint32
	for _, c := range classes {
		data := c.data
		for _, name := range c.compressors {
			var z bytes.Buffer
			w := zlib.NewWriter(&z)
			w.Write(data)
			w.Close()
			h := make([]byte, compressedHeaderSize)
			le.PutUint32(h, compressedMagic)
			le.PutUint64(h[4:], uint64(z.Len()))
			le.PutUint64(h[12:], uint64(len(data)))
			le.PutUint32(h[20:], str(name))
			data = append(h, z.Bytes()...)
		}
		compressed := 0
		if len(c.compressors) > 0 {
			compressed = len(data)
		}
		parent, base, _ := strings.Cut(c.name, "/")
		offsets = append(offsets, uint32(locs.Len()))
		for _, a := range []struct {
			kind byte
			v    uint32
		}{
			{attrModule, str("m")},
			{attrParent, str(parent)},
			{attrBase, str(base)},
			{attrExtension, str("class")},
			{attrOffset, uint32(resources.Len())},
			{attrCompressed, uint32(compressed)},
			{attrUncompressed, uint32(len(c.data))},
		} {
			locs.WriteByte(a.kind<<3 | 3)
			binary.Write(&locs, binary.BigEndian, a.v)
		}
		locs.WriteByte(attrEnd)
		resources.Write(data)
	}

	var out bytes.Buffer
	n := uint32(len(classes))
	for _, v := range []uint32{jimageMagic, 1 << 16, 0, n, n, uint32(locs.Len()), uint32(strs.Len())} {
		binary.Write(&out, le, v)
	}
	for range classes {
		binary.Write(&out, le, int32(0))
	}
	for _, off := range offsets {
		binary.Write(&out, le, off)
	}
	out.Write(locs.Bytes())
	out.Write(strs.Bytes())
	out.Write(resources.Bytes())

	if err := os.MkdirAll(filepath.Join(home, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "lib", "modules"), out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

type testImageClass struct {
	name        string
	data        []byte
	compressors []string
}

func TestCompressedJImage(t *testing.T) {
	home := t.TempDir()
	plain := []byte("\xca\xfe\xba\xbe plain class bytes")
	zipped := bytes.Repeat([]byte("\xca\xfe\xba\xbe zipped "), 50)
	twice := bytes.Repeat([]byte("\xca\xfe\xba\xbe zipped twice "), 50)
	writeJImage(t, home, []testImageClass{
		{name: "p/Plain", data: plain},
		{name: "p/Zipped", data: zipped, compressors: []string{"zip"}},
		{name: "p/Twice", data: twice, compressors: []string{"zip", "zip"}},
		{name: "p/Shared", data: plain, compressors: []string{"compact-cp"}},
	})
	p, err := Open(home, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	for name, want := range map[string][]byte{"p/Plain": plain, "p/Zipped": zipped, "p/Twice": twice} {
		got, err := p.Bytes(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		} else if !bytes.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	if _, err := p.Bytes("p/Shared"); err == nil || !strings.Contains(err.Error(), `"compact-cp"`) {
		t.Errorf("p/Shared: got error %v, want one naming compact-cp", err)
	}
}
