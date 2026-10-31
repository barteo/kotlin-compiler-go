package kmeta

import (
	"strings"
	"unicode/utf8"
)

// names turns the indices in a message into strings and class ids.
type names interface {
	str(i int) string
	// class returns a class id: packages separated by '/' and nested
	// classes by '.', as in kotlin/collections/Map.Entry.
	class(i int) string
}

// Bytes decodes the d1 argument of @kotlin.Metadata into the bytes of the
// protobuf messages (BitEncoding.decodeBytes).
func Bytes(d1 []string) []byte {
	n := 0
	for _, s := range d1 {
		n += len(s)
	}
	out := make([]byte, 0, n)
	if len(d1) > 0 && d1[0] != "" {
		r, size := utf8.DecodeRuneInString(d1[0])
		switch r {
		case 0: // UTF-8 mode: each char is a byte
			for i, s := range d1 {
				if i == 0 {
					s = s[size:]
				}
				for _, c := range s {
					out = append(out, byte(c))
				}
			}
			return out
		case 0xFFFF: // the 8-to-7 mode marker
			d1 = append([]string{d1[0][size:]}, d1[1:]...)
		}
	}
	// 8-to-7 mode: each char holds 7 bits, incremented by one.
	for _, s := range d1 {
		for _, c := range s {
			out = append(out, byte(c+0x7f)&0x7f)
		}
	}
	res := make([]byte, 7*len(out)/8)
	idx, bit := 0, 0
	for i := range res {
		first := int(out[idx]) >> bit
		idx++
		second := (int(out[idx]) & (1<<(bit+1) - 1)) << (7 - bit)
		res[i] = byte(first + second)
		if bit == 6 {
			idx++
			bit = 0
		} else {
			bit++
		}
	}
	return res
}

// jvmNames resolves strings in class file metadata: the d2 strings,
// transformed by the records of a StringTableTypes message
// (JvmNameResolverBase).
type jvmNames struct {
	strs    []string
	records []*record // one per index
}

type record struct {
	predefined int // -1 if absent
	str        *string
	substring  []int
	replace    []int
	op         int
}

const (
	opNone = iota
	opInternalToClassID
	opDescToClassID
)

// predefined is JvmNameResolverBase.PREDEFINED_STRINGS.
var predefined = []string{
	"kotlin/Any",
	"kotlin/Nothing",
	"kotlin/Unit",
	"kotlin/Throwable",
	"kotlin/Number",

	"kotlin/Byte", "kotlin/Double", "kotlin/Float", "kotlin/Int",
	"kotlin/Long", "kotlin/Short", "kotlin/Boolean", "kotlin/Char",

	"kotlin/CharSequence",
	"kotlin/String",
	"kotlin/Comparable",
	"kotlin/Enum",

	"kotlin/Array",
	"kotlin/ByteArray", "kotlin/DoubleArray", "kotlin/FloatArray", "kotlin/IntArray",
	"kotlin/LongArray", "kotlin/ShortArray", "kotlin/BooleanArray", "kotlin/CharArray",

	"kotlin/Cloneable",
	"kotlin/Annotation",

	"kotlin/collections/Iterable", "kotlin/collections/MutableIterable",
	"kotlin/collections/Collection", "kotlin/collections/MutableCollection",
	"kotlin/collections/List", "kotlin/collections/MutableList",
	"kotlin/collections/Set", "kotlin/collections/MutableSet",
	"kotlin/collections/Map", "kotlin/collections/MutableMap",
	"kotlin/collections/Map.Entry", "kotlin/collections/MutableMap.MutableEntry",

	"kotlin/collections/Iterator", "kotlin/collections/MutableIterator",
	"kotlin/collections/ListIterator", "kotlin/collections/MutableListIterator",
}

// readJVMNames reads a StringTableTypes message (jvm_metadata.proto).
func readJVMNames(msg []byte, strs []string) *jvmNames {
	n := &jvmNames{strs: strs}
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f != 1 || w != wireBytes {
			d.skip(w) // local_name
			continue
		}
		r := &record{predefined: -1}
		rng := 1
		rd := decoder{d.bytes()}
		for rd.more() {
			f, w := rd.key()
			switch f {
			case 1:
				rng = rd.int()
			case 2:
				r.predefined = rd.int()
			case 6:
				s := string(rd.bytes())
				r.str = &s
			case 3:
				r.op = rd.int()
			case 4:
				r.substring = rd.ints(w, r.substring)
			case 5:
				r.replace = rd.ints(w, r.replace)
			default:
				rd.skip(w)
			}
		}
		for range rng {
			n.records = append(n.records, r)
		}
	}
	return n
}

func (n *jvmNames) str(i int) string {
	if i < 0 || i >= len(n.records) {
		fail("string index %d out of range", i)
	}
	r := n.records[i]
	var s string
	switch {
	case r.str != nil:
		s = *r.str
	case r.predefined >= 0 && r.predefined < len(predefined):
		s = predefined[r.predefined]
	case i < len(n.strs):
		s = n.strs[i]
	default:
		fail("string index %d out of range", i)
	}
	if len(r.substring) >= 2 {
		// Indices count UTF-16 chars, which are code points in names.
		begin, end := r.substring[0], r.substring[1]
		if u := []rune(s); 0 <= begin && begin <= end && end <= len(u) {
			s = string(u[begin:end])
		}
	}
	if len(r.replace) >= 2 {
		s = strings.ReplaceAll(s, string(rune(r.replace[0])), string(rune(r.replace[1])))
	}
	switch r.op {
	case opInternalToClassID:
		s = strings.ReplaceAll(s, "$", ".")
	case opDescToClassID:
		if len(s) >= 2 {
			s = s[1 : len(s)-1]
		}
		s = strings.ReplaceAll(s, "$", ".")
	}
	return s
}

func (n *jvmNames) class(i int) string { return n.str(i) }

// tableNames resolves strings in .kotlin_builtins files: a StringTable and
// a QualifiedNameTable (NameResolverImpl).
type tableNames struct {
	strs   []string
	qnames []qname
}

type qname struct{ parent, short, kind int }

const qnameClass, qnamePackage = 0, 1

func readStringTable(msg []byte) []string {
	var strs []string
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f == 1 && w == wireBytes {
			strs = append(strs, string(d.bytes()))
		} else {
			d.skip(w)
		}
	}
	return strs
}

func readQualifiedNames(msg []byte) []qname {
	var qs []qname
	d := decoder{msg}
	for d.more() {
		f, w := d.key()
		if f != 1 || w != wireBytes {
			d.skip(w)
			continue
		}
		q := qname{parent: -1, kind: qnamePackage}
		qd := decoder{d.bytes()}
		for qd.more() {
			f, w := qd.key()
			switch f {
			case 1:
				q.parent = qd.int()
			case 2:
				q.short = qd.int()
			case 3:
				q.kind = qd.int()
			default:
				qd.skip(w)
			}
		}
		qs = append(qs, q)
	}
	return qs
}

func (n *tableNames) str(i int) string {
	if i < 0 || i >= len(n.strs) {
		fail("string index %d out of range", i)
	}
	return n.strs[i]
}

func (n *tableNames) class(i int) string {
	var pkg, cls []string
	for depth := 0; i != -1; depth++ {
		if i < 0 || i >= len(n.qnames) || depth > len(n.qnames) {
			fail("qualified name index %d out of range", i)
		}
		q := n.qnames[i]
		if q.kind == qnamePackage {
			pkg = append(pkg, n.str(q.short))
		} else {
			cls = append(cls, n.str(q.short))
		}
		i = q.parent
	}
	var b strings.Builder
	for j := len(pkg) - 1; j >= 0; j-- {
		b.WriteString(pkg[j])
		b.WriteByte('/')
	}
	for j := len(cls) - 1; j >= 0; j-- {
		b.WriteString(cls[j])
		if j > 0 {
			b.WriteByte('.')
		}
	}
	return b.String()
}
