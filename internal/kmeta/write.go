package kmeta

import (
	"encoding/binary"
	"slices"
	"sort"
)

// ModulePart is a class with a package's top-level declarations: a file
// facade, or a part of a multi-file class, whose facade's simple name is
// Facade.
type ModulePart struct {
	Name   string // simple name
	Facade string
}

// WriteModule encodes a .kotlin_module file that lists the file facades
// and multi-file class parts of a module's packages, by package name
// (dotted), as kotlinc writes it: its metadata version, flags, and a
// PackageParts message for each package, which lists the parts first,
// each with the index of its facade's name (+1).
func WriteModule(facades map[string][]ModulePart) []byte {
	var out []byte
	for _, v := range []uint32{3, 1, 9, 0, 0} { // version 1.9.0, then no flags
		out = binary.BigEndian.AppendUint32(out, v)
	}
	pkgs := make([]string, 0, len(facades))
	for p := range facades {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	for _, p := range pkgs {
		parts := append([]ModulePart(nil), facades[p]...)
		sort.Slice(parts, func(i, j int) bool {
			if (parts[i].Facade != "") != (parts[j].Facade != "") {
				return parts[i].Facade != ""
			}
			return parts[i].Name < parts[j].Name
		})
		part := field(nil, 1, []byte(p))
		var ids []byte
		var names []string
		for _, n := range parts {
			part = field(part, 2, []byte(n.Name))
			if n.Facade == "" {
				continue
			}
			k := slices.Index(names, n.Facade)
			if k < 0 {
				k = len(names)
				names = append(names, n.Facade)
			}
			ids = binary.AppendUvarint(ids, uint64(k+1))
		}
		if ids != nil {
			part = field(part, 3, ids)
		}
		for _, n := range names {
			part = field(part, 4, []byte(n))
		}
		out = field(out, 1, part)
	}
	// An empty string table and qualified name table, as kotlinc writes.
	out = field(out, 4, nil)
	return field(out, 5, nil)
}

// field appends a length-delimited protobuf field.
func field(b []byte, num int, data []byte) []byte {
	b = binary.AppendUvarint(b, uint64(num)<<3|2)
	b = binary.AppendUvarint(b, uint64(len(data)))
	return append(b, data...)
}
