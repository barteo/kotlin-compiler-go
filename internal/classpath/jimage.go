package classpath

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// The JDK stores its classes in lib/modules, a "jimage" file:
//
//	header      7 x u4: magic, version, flags, resource count, table length,
//	            locations size, strings size
//	redirect    table length x s4 (perfect hash; unused here)
//	offsets     table length x u4: offset of each location in the locations
//	locations   attribute lists, see below
//	strings     NUL-terminated names
//	resources   class file bytes, at index size + the location's offset
//
// A location is a list of attributes, each a byte (kind<<3 | length-1)
// followed by a big-endian value, ending with kind 0. The name of a class
// is /module/parent/base.extension, each part an offset into strings.
// Integers are in the platform's byte order, which is little-endian on
// every platform the JDK ships for today.
//
// A JDK linked with jlink --compress, such as the one Android Studio
// bundles, stores most classes compressed: the location's compressed size
// is nonzero, and the resource starts with a header,
//
//	magic 0xCAFEFAFA (u4), compressed size (u8), uncompressed size (u8),
//	decompressor name (u4 offset into strings), config (u4), terminal (u1)
//
// followed by the compressed bytes. Decompressing them can give another
// header, when compressions were stacked. The "zip" decompressor is zlib;
// the other one the JDK has, "compact-cp", shares constant pool strings
// across classes and is not supported.

const (
	jimageMagic = 0xCAFEDADA

	attrEnd          = 0
	attrModule       = 1
	attrParent       = 2
	attrBase         = 3
	attrExtension    = 4
	attrOffset       = 5
	attrCompressed   = 6
	attrUncompressed = 7

	compressedMagic      = 0xCAFEFAFA
	compressedHeaderSize = 29
)

type jimage struct {
	f         *os.File
	indexSize int64
	strs      []byte
}

func (j *jimage) str(off uint64) ([]byte, error) {
	if off >= uint64(len(j.strs)) {
		return nil, errors.New("bad string offset")
	}
	s := j.strs[off:]
	if end := bytes.IndexByte(s, 0); end >= 0 {
		return s[:end], nil
	}
	return nil, errors.New("unterminated string")
}

func (j *jimage) read(e *entry) ([]byte, error) {
	b := make([]byte, e.size)
	if _, err := j.f.ReadAt(b, j.indexSize+e.off); err != nil {
		return nil, err
	}
	for len(b) >= compressedHeaderSize && binary.LittleEndian.Uint32(b) == compressedMagic {
		size := binary.LittleEndian.Uint64(b[4:])
		usize := binary.LittleEndian.Uint64(b[12:])
		name, err := j.str(uint64(binary.LittleEndian.Uint32(b[20:])))
		if err != nil {
			return nil, err
		}
		if size > uint64(len(b)-compressedHeaderSize) {
			return nil, errors.New("bad compressed resource size")
		}
		if string(name) != "zip" {
			return nil, fmt.Errorf("resource compressed with %q, which is not supported", name)
		}
		r, err := zlib.NewReader(bytes.NewReader(b[compressedHeaderSize : compressedHeaderSize+size]))
		if err != nil {
			return nil, err
		}
		out := make([]byte, usize)
		if _, err := io.ReadFull(r, out); err != nil {
			return nil, fmt.Errorf("decompressing: %w", err)
		}
		b = out
	}
	return b, nil
}

func (j *jimage) close() error { return j.f.Close() }

func (p *Path) addJImage(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	j := &jimage{f: f}
	p.sources = append(p.sources, j)

	const headerSize = 28
	var hb [headerSize]byte
	if _, err := f.ReadAt(hb[:], 0); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var h [7]uint32
	for i := range h {
		h[i] = binary.LittleEndian.Uint32(hb[4*i:])
	}
	if h[0] != jimageMagic {
		return fmt.Errorf("%s: not a little-endian jimage file", path)
	}
	if major := h[1] >> 16; major != 1 {
		return fmt.Errorf("%s: unsupported jimage version %d", path, major)
	}
	tableLen, locSize, strSize := int64(h[4]), int64(h[5]), int64(h[6])
	j.indexSize = headerSize + 8*tableLen + locSize + strSize

	index := make([]byte, j.indexSize-headerSize)
	if _, err := f.ReadAt(index, headerSize); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	offsets := index[4*tableLen : 8*tableLen]
	locs := index[8*tableLen : 8*tableLen+locSize]
	j.strs = index[8*tableLen+locSize:]
	str := j.str

	var attrs [8]uint64
	var name []byte
	for i := range tableLen {
		loc := int(binary.LittleEndian.Uint32(offsets[4*i:]))
		attrs = [8]uint64{}
		for loc < len(locs) {
			b := locs[loc]
			kind := b >> 3
			if kind == attrEnd {
				break
			}
			n := int(b&7) + 1
			if loc+1+n > len(locs) {
				return fmt.Errorf("%s: bad location", path)
			}
			var v uint64
			for _, c := range locs[loc+1 : loc+1+n] {
				v = v<<8 | uint64(c)
			}
			if kind < 8 {
				attrs[kind] = v
			}
			loc += 1 + n
		}
		ext, err := str(attrs[attrExtension])
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if string(ext) != "class" {
			continue
		}
		parent, err1 := str(attrs[attrParent])
		base, err2 := str(attrs[attrBase])
		if err := errors.Join(err1, err2); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if string(base) == "module-info" {
			continue
		}
		size := attrs[attrUncompressed]
		if attrs[attrCompressed] != 0 {
			size = attrs[attrCompressed]
		}
		name = name[:0]
		if len(parent) > 0 {
			name = append(append(name, parent...), '/')
		}
		name = append(name, base...)
		p.add(string(name), entry{src: j, off: int64(attrs[attrOffset]), size: int64(size)})
	}
	return nil
}
