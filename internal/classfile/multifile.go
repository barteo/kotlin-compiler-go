package classfile

import (
	"math"
	"strings"
)

// AddDelegates adds to c, the facade of a multi-file class, a method for
// each static method of part, one of its parts, that isn't private, as
// kotlinc writes them: with the method's name, descriptor, flags,
// annotations, checked exceptions and parameter names, calling it. The
// synthetic accessors of private members, access$x, and the holders of
// properties' annotations, x$annotations, aren't delegated.
// The part's constants that aren't private, its const vals, are fields
// of the facade too.
func (c *Class) AddDelegates(part *Class) error {
	for _, f := range part.fields {
		if f.constant == 0 || f.access&AccPrivate != 0 {
			continue
		}
		c.AddConstField(f.access, part.Pool.utf8At(f.name), f.descriptor, c.Pool.copyConst(&part.Pool, f.constant))
		d := c.fields[len(c.fields)-1]
		d.anns = f.anns
		d.deprecated = f.deprecated
	}
	for _, m := range part.methods {
		if m.access&AccStatic == 0 || m.access&AccPrivate != 0 || m.code == nil ||
			m.nameStr == "<clinit>" || strings.HasPrefix(m.nameStr, "access$") || strings.HasSuffix(m.nameStr, "$annotations") {
			continue
		}
		code := c.NewMethod(m.access, m.nameStr, m.descriptor)
		d := c.methods[len(c.methods)-1]
		d.anns = m.anns
		d.params = m.params
		d.deprecated = m.deprecated
		if len(m.excNames) > 0 {
			c.Throws(m.excNames...)
		}
		code.Line(1)
		for _, pd := range descParams(m.descriptor) {
			slot := code.AllocLocal(pd)
			code.Local(kindOp(pd, ILOAD), slot)
			code.NameLocal(slot, m.code.paramName(slot))
		}
		code.Invoke(INVOKESTATIC, part.Name, m.nameStr, m.descriptor)
		switch ret := m.descriptor[strings.IndexByte(m.descriptor, ')')+1:]; ret {
		case "V":
			code.Op(RETURN)
		default:
			code.Op(kindOp(ret, IRETURN))
		}
		if err := code.Err(); err != nil {
			return err
		}
	}
	return nil
}

// paramName returns the name of the local in slot where the code starts:
// a parameter's.
func (c *Code) paramName(slot int) string {
	for _, v := range c.vars {
		if v.slot == slot && v.start == 0 {
			return v.name
		}
	}
	return ""
}

// descParams splits a method descriptor's parameter types.
func descParams(d string) []string {
	var out []string
	for i := 1; d[i] != ')'; {
		j := i
		for d[j] == '[' {
			j++
		}
		if d[j] == 'L' {
			j = i + strings.IndexByte(d[i:], ';')
		}
		out = append(out, d[i:j+1])
		i = j + 1
	}
	return out
}

// kindOp returns the instruction of the family of op, ILOAD or IRETURN,
// for a value of type descriptor d.
func kindOp(d string, op byte) byte {
	switch d[0] {
	case 'J':
		return op + 1
	case 'F':
		return op + 2
	case 'D':
		return op + 3
	case 'L', '[':
		return op + 4
	}
	return op
}

// utf8At returns the string of Utf8 constant i.
func (p *Pool) utf8At(i uint16) string {
	for s, j := range p.utf8 {
		if j == i {
			return s
		}
	}
	return ""
}

// copyConst adds to p constant i of pool from, a ConstantValue: a
// number or a string.
func (p *Pool) copyConst(from *Pool, i uint16) uint16 {
	for k, j := range from.index {
		if j != i {
			continue
		}
		switch k.tag {
		case tagInteger:
			return p.Int(int32(uint32(k.n)))
		case tagFloat:
			return p.Float(math.Float32frombits(uint32(k.n)))
		case tagLong:
			return p.Long(int64(k.n))
		case tagDouble:
			return p.Double(math.Float64frombits(k.n))
		case tagString:
			return p.String(k.a)
		}
	}
	panic("classfile: no constant to copy")
}
