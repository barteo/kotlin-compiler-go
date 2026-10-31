package classfile

import "fmt"

// Annotation is an annotation in a class file: its type's descriptor
// (Lkotlin/Deprecated;) and its elements.
type Annotation struct {
	Type   string
	Values []ElementPair
}

// ElementPair is an annotation's element: its name and value.
type ElementPair struct {
	Name  string
	Value ElementValue
}

// ElementValue is the value of an annotation's element. Tag is one of
// B C D F I J S Z (a constant in Const: an int64 for the integral types
// and Z, a float32 for F, a float64 for D), s (a string in Const), e (an
// enum constant: Enum's type descriptor and name), c (a class: Class is
// its return descriptor), @ (an annotation) or [ (an array of Elems).
type ElementValue struct {
	Tag   byte
	Const any
	Enum  [2]string
	Class string
	Ann   *Annotation
	Elems []ElementValue
}

func (c *Class) annotation(b []byte, a Annotation) []byte {
	b = u2(b, c.Utf8(a.Type))
	b = u2(b, uint16(len(a.Values)))
	for _, p := range a.Values {
		b = u2(b, c.Utf8(p.Name))
		b = c.elementValue(b, p.Value)
	}
	return b
}

func (c *Class) elementValue(b []byte, v ElementValue) []byte {
	b = append(b, v.Tag)
	switch v.Tag {
	case 'B', 'C', 'I', 'S', 'Z':
		return u2(b, c.Int(int32(v.Const.(int64))))
	case 'J':
		return u2(b, c.Long(v.Const.(int64)))
	case 'F':
		return u2(b, c.Float(v.Const.(float32)))
	case 'D':
		return u2(b, c.Double(v.Const.(float64)))
	case 's':
		return u2(b, c.Utf8(v.Const.(string)))
	case 'e':
		return u2(u2(b, c.Utf8(v.Enum[0])), c.Utf8(v.Enum[1]))
	case 'c':
		return u2(b, c.Utf8(v.Class))
	case '@':
		return c.annotation(b, *v.Ann)
	case '[':
		b = u2(b, uint16(len(v.Elems)))
		for _, e := range v.Elems {
			b = c.elementValue(b, e)
		}
		return b
	}
	panic(fmt.Sprintf("element value tag %q", v.Tag))
}
