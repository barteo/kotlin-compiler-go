package kmeta

import (
	"encoding/binary"
	"math"
	"strings"
)

// Encoding @kotlin.Metadata: the inverse of the decoders, writing the
// messages of metadata.proto and jvm_metadata.proto as kotlinc does,
// fields in their numbers' order and default values left out.

// Header is the arguments of a @kotlin.Metadata annotation.
type Header struct {
	K  int
	MV []int
	XI int
	D1 []string
	D2 []string
	XS string // a multi-file part's facade
}

// Version is the metadata version kc writes, which kotlinc 1.9 and later
// read, the module file's.
var Version = []int{1, 9, 0}

// XI is the extra flags kotlinc 2.x writes: 16 for code compiled by the
// JVM IR backend, 32 for its stable ABI.
const XI = 48

// XIPublicABI marks a class of the body of an inline function that other
// modules copy, one that isn't private: kotlinc's inliner, run with
// assertions, refuses to copy an anonymous class without it.
const XIPublicABI = 128

// enc appends protobuf fields to b.
type enc struct{ b []byte }

func (e *enc) key(field, wire int) { e.b = binary.AppendUvarint(e.b, uint64(field)<<3|uint64(wire)) }

// int writes an int32 field: a negative value takes ten bytes, as
// protobuf sign-extends it.
func (e *enc) int(field, v int) {
	e.key(field, wireVarint)
	e.b = binary.AppendUvarint(e.b, uint64(int64(int32(v))))
}

func (e *enc) bool(field int, v bool) {
	if v {
		e.int(field, 1)
	}
}

func (e *enc) msg(field int, b []byte) {
	e.key(field, wireBytes)
	e.b = binary.AppendUvarint(e.b, uint64(len(b)))
	e.b = append(e.b, b...)
}

func (e *enc) packed(field int, vs []int) {
	if len(vs) == 0 {
		return
	}
	var p []byte
	for _, v := range vs {
		p = binary.AppendUvarint(p, uint64(int64(int32(v))))
	}
	e.msg(field, p)
}

// stringTable builds the strings a message refers to: the d2 strings and
// the records of a StringTableTypes message (JvmStringTable).
type stringTable struct {
	strs    []string // d2
	records []strRecord
	index   map[strKey]int
	local   []int // the indices of local class names
}

type strKey struct {
	s     string
	class bool
}

type strRecord struct {
	predefined int // -1 if none
	op         int
}

func newStringTable() *stringTable { return &stringTable{index: map[strKey]int{}} }

// str returns the index of a string.
func (t *stringTable) str(s string) int {
	k := strKey{s, false}
	if i, ok := t.index[k]; ok {
		return i
	}
	i := len(t.strs)
	t.strs = append(t.strs, s)
	t.records = append(t.records, strRecord{predefined: -1})
	t.index[k] = i
	return i
}

// class returns the index of a class id, kotlin/collections/Map.Entry: a
// predefined one, or its descriptor with DESC_TO_CLASS_ID, as kotlinc
// writes it.
func (t *stringTable) class(id string) int {
	k := strKey{id, true}
	if i, ok := t.index[k]; ok {
		return i
	}
	r := strRecord{predefined: -1}
	s := ""
	for j, p := range predefined {
		if p == id {
			r.predefined = j
		}
	}
	switch {
	case r.predefined >= 0:
	case strings.Contains(id, "$"):
		// A local or anonymous class, whose name keeps its '$'s: a plain
		// string, listed as a local name.
		s = id
		t.local = append(t.local, len(t.strs))
	default:
		r.op = opDescToClassID
		s = "L" + strings.ReplaceAll(id, ".", "$") + ";"
	}
	i := len(t.strs)
	t.strs = append(t.strs, s)
	t.records = append(t.records, r)
	t.index[k] = i
	return i
}

// types encodes the StringTableTypes message: records, with runs of equal
// ones in one record with a range.
func (t *stringTable) types() []byte {
	var e enc
	for i := 0; i < len(t.records); {
		r := t.records[i]
		n := 1
		for i+n < len(t.records) && t.records[i+n] == r {
			n++
		}
		var re enc
		if n > 1 {
			re.int(1, n)
		}
		if r.predefined >= 0 {
			re.int(2, r.predefined)
		}
		if r.op != opNone {
			re.int(3, r.op)
		}
		e.msg(1, re.b)
		i += n
	}
	e.packed(5, t.local)
	return e.b
}

// encodeD1 encodes bytes as the d1 strings, in BitEncoding's UTF-8 mode:
// a leading U+0000, then one char per byte, split so that each string's
// modified UTF-8 takes at most 65535 bytes, as a class file's Utf8
// constants must.
func encodeD1(b []byte) []string {
	var out []string
	var cur []rune
	size := 0
	flush := func() {
		out = append(out, string(cur))
		cur, size = nil, 0
	}
	add := func(r rune) {
		n := 1
		if r == 0 || r >= 0x80 {
			n = 2
		}
		if size+n > 65535 {
			flush()
		}
		cur = append(cur, r)
		size += n
	}
	add(0)
	for _, c := range b {
		add(rune(c))
	}
	flush()
	return out
}

// writer encodes messages, with the string table their indices refer to.
type writer struct{ st *stringTable }

func (w *writer) header(k int, msg []byte) Header {
	stt := w.st.types()
	b := binary.AppendUvarint(nil, uint64(len(stt)))
	b = append(b, stt...)
	b = append(b, msg...)
	d2 := w.st.strs
	if d2 == nil {
		d2 = []string{}
	}
	return Header{K: k, MV: Version, XI: XI, D1: encodeD1(b), D2: d2}
}

// WriteClass encodes a class's metadata (kind 1).
func WriteClass(c *Class) Header {
	w := &writer{newStringTable()}
	return w.header(KindClassFile, w.class(c))
}

// WritePackage encodes a file facade's metadata (kind 2).
func WritePackage(p *PackageData) Header {
	w := &writer{newStringTable()}
	var e enc
	for _, f := range p.Funcs {
		e.msg(3, w.function(f))
	}
	for _, pr := range p.Props {
		e.msg(4, w.property(pr))
	}
	for _, a := range p.Aliases {
		e.msg(5, w.typeAlias(a))
	}
	if p.ModuleName != "" {
		e.int(101, w.st.str(p.ModuleName))
	}
	return w.header(KindFileFacade, e.b)
}

// WriteSynthetic returns the metadata of a synthetic class (kind 3),
// which has no messages.
func WriteSynthetic() Header { return Header{K: KindSynthetic, MV: Version, XI: XI} }

// XILocal is the extra flag kotlinc writes on the synthetic classes of
// local code: continuations and lambdas.
const XILocal = 512

// WriteLocalSynthetic returns the metadata of a synthetic class of local
// code, such as a continuation class.
func WriteLocalSynthetic() Header { return Header{K: KindSynthetic, MV: Version, XI: XI | XILocal} }

// XILambda are the extra flags kotlinc writes on a lambda's class.
const XILambda = XI | 256 | 1024

// XIInlineLambda are the extra flags kotlinc 2.4 writes on the class of a
// lambda, or of a SAM conversion, in the body of an inline function other
// modules copy: those of a lambda, with 512 in place of 1024, and
// XIPublicABI.
const XIInlineLambda = XI | XIPublicABI | 256 | XILocal

// WriteLambda encodes the metadata of a lambda's class (kind 3): its
// function, as kotlinc writes it, named <anonymous>, without a JVM
// signature.
func WriteLambda(f *Function) Header {
	w := &writer{newStringTable()}
	types := make([]*Type, 0, len(f.Params)+1)
	if f.Recv != nil {
		types = append(types, f.Recv)
	}
	for _, p := range f.Params {
		types = append(types, p.Type)
	}
	f.JVM = JVMSig{Name: f.Name, Desc: defaultDesc(types, f.Ret)}
	h := w.header(KindSynthetic, w.function(f))
	h.XI = XILambda
	return h
}

func (w *writer) class(c *Class) []byte {
	var e enc
	if c.Flags != 6 {
		e.int(1, int(c.Flags))
	}
	e.int(3, w.st.class(c.Name))
	if c.Companion != "" {
		e.int(4, w.st.str(c.Companion))
	}
	for _, p := range c.TypeParams {
		e.msg(5, w.typeParam(p))
	}
	for _, t := range c.Supers {
		e.msg(6, w.typ(t))
	}
	if len(c.Nested) > 0 {
		ids := make([]int, len(c.Nested))
		for i, n := range c.Nested {
			ids[i] = w.st.str(n)
		}
		e.packed(7, ids)
	}
	for _, k := range c.Ctors {
		e.msg(8, w.constructor(k))
	}
	for _, f := range c.Funcs {
		e.msg(9, w.function(f))
	}
	for _, p := range c.Props {
		e.msg(10, w.property(p))
	}
	for _, a := range c.Aliases {
		e.msg(11, w.typeAlias(a))
	}
	for _, n := range c.EnumEntries {
		var ee enc
		ee.int(1, w.st.str(n))
		e.msg(13, ee.b)
	}
	if len(c.SealedSubclasses) > 0 {
		ids := make([]int, len(c.SealedSubclasses))
		for i, n := range c.SealedSubclasses {
			ids[i] = w.st.class(n)
		}
		e.packed(16, ids)
	}
	if c.UnderlyingProp != "" {
		e.int(17, w.st.str(c.UnderlyingProp))
	}
	if c.Underlying != nil {
		e.msg(18, w.typ(c.Underlying))
	}
	if c.ModuleName != "" {
		e.int(101, w.st.str(c.ModuleName))
	}
	if c.JVMFlags != 0 {
		e.int(104, c.JVMFlags)
	}
	for _, n := range c.SerialProps {
		e.int(18000, w.st.str(n)) // SerializationPluginMetadataExtensions.propertiesNamesInProgramOrder
	}
	return e.b
}

func (w *writer) constructor(k *Constructor) []byte {
	var e enc
	if k.Flags != 6 {
		e.int(1, int(k.Flags))
	}
	for _, p := range k.Params {
		e.msg(2, w.param(p))
	}
	types := make([]*Type, len(k.Params))
	for i, p := range k.Params {
		types[i] = p.Type
	}
	var s enc
	if k.JVM.Name != "<init>" {
		s.int(1, w.st.str(k.JVM.Name))
	}
	if d := defaultDesc(types, &Type{Class: "kotlin/Unit"}); k.JVM.Desc != d || d == "" {
		s.int(2, w.st.str(k.JVM.Desc))
	}
	if s.b != nil {
		e.msg(100, s.b)
	}
	return e.b
}

func (w *writer) function(f *Function) []byte {
	var e enc
	e.int(2, w.st.str(f.Name))
	e.msg(3, w.typ(f.Ret))
	for _, p := range f.TypeParams {
		e.msg(4, w.typeParam(p))
	}
	if f.Recv != nil {
		e.msg(5, w.typ(f.Recv))
	}
	for _, p := range f.Params {
		e.msg(6, w.param(p))
	}
	if f.Flags != 6 {
		e.int(9, int(f.Flags))
	}
	for _, p := range f.ContextParams {
		e.msg(10, w.typ(p.Type))
	}
	for _, a := range f.Annotations {
		var x enc
		x.int(1, w.st.class(a))
		e.msg(12, x.b)
	}
	for _, p := range f.ContextParams {
		e.msg(13, w.param(p))
	}
	if len(f.InPlace) > 0 || len(f.Implies) > 0 {
		var c enc
		for _, im := range f.Implies {
			var conclusion, eff enc
			if im.Is == nil {
				conclusion.int(1, 3) // negated null check: p != null
			}
			conclusion.int(2, im.Param+1)
			if im.Is != nil {
				conclusion.msg(4, w.typ(im.Is))
			}
			eff.int(1, 0) // RETURNS_CONSTANT
			if im.Returns != ReturnsAny {
				var x enc
				x.int(3, im.Returns)
				eff.msg(2, x.b)
			}
			eff.msg(3, conclusion.b)
			c.msg(1, eff.b)
		}
		for _, ip := range f.InPlace {
			var x, eff enc
			x.int(2, ip.Param+1)
			eff.int(1, 1) // CALLS
			eff.msg(2, x.b)
			if ip.Kind != Unknown {
				eff.int(4, ip.Kind)
			}
			c.msg(1, eff.b)
		}
		e.msg(32, c.b)
	}
	for _, pd := range f.PluginData {
		var x enc
		x.int(1, w.st.str(pd.Plugin))
		x.msg(2, pd.Data)
		e.msg(33, x.b)
	}
	types := make([]*Type, 0, len(f.Params)+1)
	if f.Recv != nil {
		types = append(types, f.Recv)
	}
	for _, p := range f.Params {
		types = append(types, p.Type)
	}
	var s enc
	if f.JVM.Name != f.Name {
		s.int(1, w.st.str(f.JVM.Name))
	}
	if d := defaultDesc(types, f.Ret); f.JVM.Desc != d || d == "" {
		s.int(2, w.st.str(f.JVM.Desc))
	}
	if s.b != nil {
		e.msg(100, s.b)
	}
	return e.b
}

func (w *writer) property(p *Property) []byte {
	var e enc
	e.int(2, w.st.str(p.Name))
	e.msg(3, w.typ(p.Type))
	for _, tp := range p.TypeParams {
		e.msg(4, w.typeParam(tp))
	}
	if p.Recv != nil {
		e.msg(5, w.typ(p.Recv))
	}
	if p.SetterParam != nil {
		e.msg(6, w.param(p.SetterParam))
	}
	if p.GetterFlags != p.Flags&0x3f {
		e.int(7, int(p.GetterFlags))
	}
	if p.SetterFlags != p.Flags&0x3f {
		e.int(8, int(p.SetterFlags))
	}
	if p.Flags != 518 {
		e.int(11, int(p.Flags))
	}
	for _, c := range p.ContextParams {
		e.msg(12, w.typ(c.Type))
	}
	for _, c := range p.ContextParams {
		e.msg(17, w.param(c))
	}
	var sig enc
	if p.Field != nil {
		var s enc
		if p.Field.Name != p.Name {
			s.int(1, w.st.str(p.Field.Name))
		}
		if d := mapTypeDefault(p.Type); p.Field.Desc != d || d == "" {
			s.int(2, w.st.str(p.Field.Desc))
		}
		sig.msg(1, s.b)
	}
	method := func(field int, m *JVMSig) {
		if m != nil {
			var s enc
			s.int(1, w.st.str(m.Name))
			s.int(2, w.st.str(m.Desc))
			sig.msg(field, s.b)
		}
	}
	method(2, p.Synthetic)
	method(3, p.Getter)
	method(4, p.Setter)
	if sig.b != nil {
		e.msg(100, sig.b)
	}
	return e.b
}

func (w *writer) typeAlias(a *TypeAlias) []byte {
	var e enc
	if a.Flags != 6 {
		e.int(1, int(a.Flags))
	}
	e.int(2, w.st.str(a.Name))
	for _, p := range a.TypeParams {
		e.msg(3, w.typeParam(p))
	}
	if a.Underlying != nil {
		e.msg(4, w.typ(a.Underlying))
	}
	if a.Expanded != nil {
		e.msg(6, w.typ(a.Expanded))
	}
	for _, an := range a.Annotations {
		e.msg(8, w.annotation(an))
	}
	return e.b
}

func (w *writer) annotation(a *Annotation) []byte {
	var e enc
	e.int(1, w.st.class(a.Class))
	for _, arg := range a.Args {
		var x enc
		x.int(1, w.st.str(arg.Name))
		x.msg(2, w.annotationValue(arg.Value))
		e.msg(2, x.b)
	}
	return e.b
}

func (w *writer) annotationValue(v AnnotationValue) []byte {
	var e enc
	e.int(1, int(v.Kind))
	switch v.Kind {
	case ValueByte, ValueChar, ValueShort, ValueInt, ValueLong, ValueBoolean:
		e.key(2, wireVarint) // sint64
		e.b = binary.AppendUvarint(e.b, uint64(v.Int<<1^v.Int>>63))
	case ValueFloat:
		e.key(3, wire32)
		e.b = binary.LittleEndian.AppendUint32(e.b, math.Float32bits(float32(v.Float)))
	case ValueDouble:
		e.key(4, wire64)
		e.b = binary.LittleEndian.AppendUint64(e.b, math.Float64bits(v.Float))
	case ValueString:
		e.int(5, w.st.str(v.Str))
	case ValueClass:
		e.int(6, w.st.class(v.Class))
	case ValueEnum:
		e.int(6, w.st.class(v.Class))
		e.int(7, w.st.str(v.Str))
	case ValueAnnotation:
		e.msg(8, w.annotation(v.Ann))
	case ValueArray:
		for _, el := range v.Elems {
			e.msg(9, w.annotationValue(el))
		}
	}
	return e.b
}

func (w *writer) param(p *Param) []byte {
	var e enc
	if p.Flags != 0 {
		e.int(1, p.Flags)
	}
	e.int(2, w.st.str(p.Name))
	e.msg(3, w.typ(p.Type))
	if p.Vararg != nil {
		e.msg(4, w.typ(p.Vararg))
	}
	return e.b
}

func (w *writer) typeParam(p *TypeParam) []byte {
	var e enc
	e.int(1, p.ID)
	e.int(2, w.st.str(p.Name))
	e.bool(3, p.Reified)
	if p.Variance != Inv {
		e.int(4, int(p.Variance))
	}
	for _, b := range p.Bounds {
		e.msg(5, w.typ(b))
	}
	return e.b
}

// platformType is the capabilities id of a flexible type that is a Java
// platform type.
const platformType = "kotlin.jvm.PlatformType"

func (w *writer) typ(t *Type) []byte {
	var e enc
	if t.Flags != 0 {
		e.int(1, t.Flags)
	}
	for _, a := range t.Args {
		var ae enc
		if a.Proj != Inv {
			ae.int(1, int(a.Proj))
		}
		if a.Type != nil {
			ae.msg(2, w.typ(a.Type))
		}
		e.msg(2, ae.b)
	}
	e.bool(3, t.Nullable)
	if t.Upper != nil {
		e.int(4, w.st.str(platformType))
		e.msg(5, w.typ(t.Upper))
	}
	switch {
	case t.Class != "":
		e.int(6, w.st.class(t.Class))
	case t.ParamName != "":
		e.int(9, w.st.str(t.ParamName))
	default:
		e.int(7, t.Param)
	}
	if t.Outer != nil {
		e.msg(10, w.typ(t.Outer))
	}
	if t.Composable {
		var a enc
		a.int(1, w.st.class("androidx/compose/runtime/Composable"))
		e.msg(100, a.b)
	}
	if t.ExtFunc {
		var a enc
		a.int(1, w.st.class("kotlin/ExtensionFunctionType"))
		e.msg(100, a.b)
	}
	if t.NoInfer {
		var a enc
		a.int(1, w.st.class("kotlin/internal/NoInfer"))
		e.msg(100, a.b)
	}
	if t.Contexts > 0 {
		// @ContextFunctionTypeParams(count = n): an argument whose value
		// is an INT, in zigzag encoding.
		var a, arg, v enc
		a.int(1, w.st.class("kotlin/ContextFunctionTypeParams"))
		v.int(1, 3)
		v.int(2, t.Contexts<<1)
		arg.int(1, w.st.str("count"))
		arg.msg(2, v.b)
		a.msg(2, arg.b)
		e.msg(100, a.b)
	}
	return e.b
}
