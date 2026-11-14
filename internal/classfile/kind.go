package classfile

import "strings"

// Kind describes what instructions do with values of a JVM type, by the
// first byte of its descriptor.
type Kind struct {
	Load, Store, Return byte // ILOAD, ISTORE, IRETURN and the others
	ALoad, AStore       byte // the array element's IALOAD, IASTORE...
	NewArray            byte // NEWARRAY's operand, TInt..., 0 for a reference
	Size                int  // the stack and local slots it takes
}

// kinds are the kinds by descriptor byte: a reference's for any byte but
// a primitive's or V's.
var kinds = func() (t [128]Kind) {
	for i := range t {
		t[i] = Kind{ALOAD, ASTORE, ARETURN, AALOAD, AASTORE, 0, 1}
	}
	t['Z'] = Kind{ILOAD, ISTORE, IRETURN, BALOAD, BASTORE, TBoolean, 1}
	t['B'] = Kind{ILOAD, ISTORE, IRETURN, BALOAD, BASTORE, TByte, 1}
	t['C'] = Kind{ILOAD, ISTORE, IRETURN, CALOAD, CASTORE, TChar, 1}
	t['S'] = Kind{ILOAD, ISTORE, IRETURN, SALOAD, SASTORE, TShort, 1}
	t['I'] = Kind{ILOAD, ISTORE, IRETURN, IALOAD, IASTORE, TInt, 1}
	t['J'] = Kind{LLOAD, LSTORE, LRETURN, LALOAD, LASTORE, TLong, 2}
	t['F'] = Kind{FLOAD, FSTORE, FRETURN, FALOAD, FASTORE, TFloat, 1}
	t['D'] = Kind{DLOAD, DSTORE, DRETURN, DALOAD, DASTORE, TDouble, 2}
	t['V'].Return, t['V'].Size = RETURN, 0
	return t
}()

// KindOf returns the kind of values of type descriptor d, a reference's
// for "".
func KindOf(d string) *Kind {
	if d == "" {
		return &kinds['L']
	}
	return &kinds[d[0]&127]
}

// DescParams splits a method descriptor's parameter types.
func DescParams(d string) []string {
	var out []string
	for i := 1; d[i] != ')'; {
		j := i
		for d[j] == '[' {
			j++
		}
		if d[j] == 'L' {
			j += strings.IndexByte(d[j:], ';')
		}
		out = append(out, d[i:j+1])
		i = j + 1
	}
	return out
}

// RetDesc returns the descriptor of a method descriptor's result type.
func RetDesc(d string) string { return d[strings.IndexByte(d, ')')+1:] }
