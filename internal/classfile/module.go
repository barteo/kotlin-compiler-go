package classfile

import "errors"

// ModuleExports reads a module-info class file: the module's name, and
// the packages it exports to all modules, by internal name (java/util).
// Exports to named modules only aren't listed.
func ModuleExports(data []byte) (string, []string, error) {
	r := &reader{b: data}
	p, err := readPool(r)
	if err != nil {
		return "", nil, err
	}
	// name_index of a Module or Package entry.
	named := func(i uint16, tag byte) (string, error) {
		if int(i) >= len(p.tags) || p.tags[i] != tag {
			return "", errors.New("bad module-info constant")
		}
		return p.utf8(uint16(p.b[p.offs[i]])<<8 | uint16(p.b[p.offs[i]+1]))
	}
	r.skip(6) // access, this_class, super_class
	if r.u2() != 0 {
		return "", nil, errors.New("module-info with interfaces")
	}
	for range 2 { // fields and methods
		for range r.u2() {
			r.skip(6)
			for range r.u2() {
				r.skip(2)
				r.skip(int(r.u4()))
			}
		}
	}
	for range r.u2() {
		attr, err := p.utf8(r.u2())
		if err != nil {
			return "", nil, err
		}
		size := int(r.u4())
		if attr != "Module" {
			r.skip(size)
			continue
		}
		module, err := named(r.u2(), 19)
		if err != nil {
			return "", nil, err
		}
		r.skip(4)               // flags, version
		r.skip(6 * int(r.u2())) // requires
		var exports []string
		for range r.u2() {
			pkg, err := named(r.u2(), 20)
			if err != nil {
				return "", nil, err
			}
			r.skip(2) // flags
			to := int(r.u2())
			r.skip(2 * to)
			if to == 0 {
				exports = append(exports, pkg)
			}
		}
		if r.err != nil {
			return "", nil, r.err
		}
		return module, exports, nil
	}
	if r.err != nil {
		return "", nil, r.err
	}
	return "", nil, errors.New("no Module attribute")
}
