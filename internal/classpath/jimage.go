package classpath

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
)

type jimage struct {
	f         *os.File
	indexSize int64
}

func (j *jimage) read(e *entry) ([]byte, error) {
	b := make([]byte, e.size)
	_, err := j.f.ReadAt(b, j.indexSize+e.off)
	return b, err
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
	strs := index[8*tableLen+locSize:]
	str := func(off uint64) ([]byte, error) {
		if off >= uint64(len(strs)) {
			return nil, errors.New("bad string offset")
		}
		s := strs[off:]
		if end := bytes.IndexByte(s, 0); end >= 0 {
			return s[:end], nil
		}
		return nil, errors.New("unterminated string")
	}

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
		if attrs[attrCompressed] != 0 {
			return fmt.Errorf("%s: compressed images are not supported", path)
		}
		name = name[:0]
		if len(parent) > 0 {
			name = append(append(name, parent...), '/')
		}
		name = append(name, base...)
		p.add(string(name), entry{src: j, off: int64(attrs[attrOffset]), size: int64(attrs[attrUncompressed])})
	}
	return nil
}
