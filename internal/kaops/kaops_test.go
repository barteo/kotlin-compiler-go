package kaops

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
	"github.com/barteo/kotlin-compiler-go/internal/kcabi/wire"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func TestOpenUpdateClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.kt")
	if err := os.WriteFile(path, []byte("package p\nannotation class A\n@A class C\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	var w wire.Writer
	w.String("")
	w.Bool(true) // no JDK
	w.String("m")
	w.Uint(1)
	w.Uint(0) // no class path
	w.Uint(0) // no friend paths
	resp, err := kcabi.Call(OpKaOpen, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	session := wire.NewReader(resp).Int()

	w = wire.Writer{}
	w.Int(session)
	w.Uint(1)
	w.String(path)
	w.Uint(0) // nothing kept
	resp, err = kcabi.Call(OpKaUpdate, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	r := wire.NewReader(resp)
	model := r.Int()
	check := r.Uint()
	if r.Bool() {
		t.Fatal("same as a kept check")
	}
	if n := r.Uint(); n != 1 {
		t.Fatalf("%d files", n)
	}
	if p, pkg := r.String(), r.String(); p != path || pkg != "p" {
		t.Fatalf("file %s, package %s", p, pkg)
	}
	if n := r.Uint(); n != 0 {
		t.Fatalf("%d file annotations", n)
	}
	if decls := readDecls(r); len(decls) != 2 {
		t.Fatalf("%d declarations", len(decls))
	}
	var lookups []ka.Lookup
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		lookups = append(lookups, ka.Lookup{Scope: r.String(), Kind: ka.LookupKind(r.Uint()), Name: r.String()})
	}
	// @A: the annotation class, by its simple name in the file's package.
	if !slices.Contains(lookups, ka.Lookup{Scope: "p", Kind: ka.PackageLookup, Name: "A"}) {
		t.Errorf("lookups %v: no A in p", lookups)
	}
	if nodes, kinds := r.Uint(), r.Uint(); nodes < 10 || kinds == 0 {
		t.Errorf("nodes %d, kinds %x", nodes, kinds)
	}
	if n := r.Uint(); n != 0 {
		t.Errorf("%d diagnostics", n)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if r.Len() != 0 {
		t.Error("bytes left after the model")
	}

	// The same update again, with the first model's check kept: the same
	// check, without the files.
	w = wire.Writer{}
	w.Int(session)
	w.Uint(1)
	w.String(path)
	w.Uint(1)
	w.Uint(check)
	resp, err = kcabi.Call(OpKaUpdate, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	r = wire.NewReader(resp)
	again := r.Int()
	if c, same := r.Uint(), r.Bool(); c != check || !same || r.Len() != 0 {
		t.Errorf("check %d, same %v, %d bytes; want check %d, same, nothing more", c, same, r.Len(), check)
	}

	for _, h := range []int64{model, again, session} {
		w = wire.Writer{}
		w.Int(h)
		if _, err := kcabi.Call(OpKaClose, w.Message()); err != nil {
			t.Fatal(err)
		}
	}
	if handles.Len() != 0 {
		t.Errorf("%d handles left", handles.Len())
	}
	w = wire.Writer{}
	w.Int(session)
	w.Uint(0)
	if _, err := kcabi.Call(OpKaUpdate, w.Message()); err == nil {
		t.Error("update of a closed session succeeded")
	}
}

// readValue reads a value as jvm/kc-aa's KcModel does.
func readValue(r *wire.Reader) *ka.Value {
	kind := ka.ValueKind(r.Uint())
	if kind == 0 {
		return nil
	}
	v := &ka.Value{Kind: kind, Node: psi.NodeID(r.Uint())}
	switch kind {
	case ka.ValueConst:
		v.Const = ka.ConstKind(r.Uint())
		switch v.Const {
		case ka.ConstNull:
		case ka.ConstFloat, ka.ConstDouble:
			v.Float = math.Float64frombits(r.Uint())
		case ka.ConstString:
			v.Str = r.String()
		default:
			v.Bits = r.Int()
		}
	case ka.ValueEnum:
		v.ClassID, v.Entry = r.String(), r.String()
	case ka.ValueClass:
		if r.Uint() != uint64(ka.TypeClass) {
			r.Int() // fail the test below
		}
		v.Type = &ka.Type{Kind: ka.TypeClass, ClassID: r.String()}
		r.Uint()       // flags
		r.Uint()       // param
		_ = r.String() // name
		r.Uint()       // contexts
		r.Uint()       // no arguments
		r.Uint()       // no abbreviation
		r.Uint()       // no annotations
	case ka.ValueAnnotation:
		v.Annotation = readAnnotation(r)
	case ka.ValueArray:
		for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
			v.Elems = append(v.Elems, readValue(r))
		}
	}
	return v
}

func readAnnotation(r *wire.Reader) *ka.Annotation {
	a := &ka.Annotation{ClassID: r.String(), Node: psi.NodeID(r.Uint()), Target: r.String(), Implied: r.Uint() == 1}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		a.Args = append(a.Args, ka.NamedValue{Name: r.String(), Value: readValue(r)})
	}
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		a.TypeArgs = append(a.TypeArgs, readType(r))
	}
	return a
}

func TestValues(t *testing.T) {
	in := &ka.Annotation{ClassID: "p/A", Node: 3, Target: "field", Implied: true, Args: []ka.NamedValue{
		{Name: "s", Value: &ka.Value{Kind: ka.ValueConst, Node: 4, Const: ka.ConstString, Str: "x"}},
		{Name: "i", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstInt, Bits: -4}},
		{Name: "u", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstULong, Bits: -1}},
		{Name: "d", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstDouble, Float: 0.1}},
		{Name: "n", Value: &ka.Value{Kind: ka.ValueConst, Const: ka.ConstNull}},
		{Name: "e", Value: &ka.Value{Kind: ka.ValueEnum, Node: 5, ClassID: "p/E", Entry: "B"}},
		{Name: "k", Value: &ka.Value{Kind: ka.ValueClass, Type: &ka.Type{Kind: ka.TypeClass, ClassID: "kotlin/String"}}},
		{Name: "a", Value: &ka.Value{Kind: ka.ValueArray, Elems: []*ka.Value{
			{Kind: ka.ValueAnnotation, Node: 6, Annotation: &ka.Annotation{ClassID: "p/B", Node: 6}},
			{Kind: ka.ValueUnsupported, Node: 7},
		}}},
	}, TypeArgs: []*ka.Type{{Kind: ka.TypeClass, ClassID: "kotlin/String"}, nil}}
	var w wire.Writer
	writeAnnotation(&w, in)
	writeValue(&w, nil)
	r := wire.NewReader(w.Message())
	out := readAnnotation(r)
	if v := readValue(r); v != nil {
		t.Errorf("none read as %v", v)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("read %s, want %s", out, in)
	}
}

func TestClass(t *testing.T) {
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skipf("no kotlin-stdlib.jar (set KOTLIN_HOME): %v", err)
	}
	path := filepath.Join(t.TempDir(), "a.kt")
	if err := os.WriteFile(path, []byte("package p\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	var w wire.Writer
	w.String("")
	w.Bool(true)
	w.String("m")
	w.Uint(1)
	w.Uint(1)
	w.String(jar)
	w.Uint(0) // no friend paths
	resp, err := kcabi.Call(OpKaOpen, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	session := wire.NewReader(resp).Int()
	w = wire.Writer{}
	w.Int(session)
	w.Uint(1)
	w.String(path)
	w.Uint(0) // nothing kept
	resp, err = kcabi.Call(OpKaUpdate, w.Message())
	if err != nil {
		t.Fatal(err)
	}
	model := wire.NewReader(resp).Int()

	for _, tc := range []struct {
		id   string
		n    uint64
		kind ka.Kind
	}{{"kotlin/Deprecated", 1, ka.KindAnnotationClass}, {"no/Such", 0, 0}} {
		w = wire.Writer{}
		w.Int(model)
		w.String(tc.id)
		resp, err = kcabi.Call(OpKaClass, w.Message())
		if err != nil {
			t.Fatal(err)
		}
		r := wire.NewReader(resp)
		if n := r.Uint(); n != tc.n {
			t.Fatalf("%s: %d decls", tc.id, n)
		}
		if tc.n == 0 {
			continue
		}
		if k, name := ka.Kind(r.Uint()), r.String(); k != tc.kind || name != "Deprecated" {
			t.Errorf("%s: kind %d, name %s", tc.id, k, name)
		}
	}
	for _, tc := range []struct {
		name   string
		exists bool
	}{{"kotlin.collections", true}, {"kotlin.no.such", false}} {
		w = wire.Writer{}
		w.Int(model)
		w.String(tc.name)
		resp, err = kcabi.Call(OpKaPackage, w.Message())
		if err != nil {
			t.Fatal(err)
		}
		r := wire.NewReader(resp)
		if exists := r.Bool(); exists != tc.exists {
			t.Errorf("%s: exists %v", tc.name, exists)
		}
		var classes []string
		for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
			classes = append(classes, r.String())
		}
		decls := readDecls(r)
		if err := r.Err(); err != nil || r.Len() != 0 {
			t.Fatalf("%s: %v, %d bytes left", tc.name, err, r.Len())
		}
		if !tc.exists {
			if len(classes) != 0 || len(decls) != 0 {
				t.Errorf("%s: %d classes, %d decls", tc.name, len(classes), len(decls))
			}
			continue
		}
		var found bool
		for _, d := range decls {
			if d.Name == "arrayListOf" && len(d.Params) == 0 {
				found = true
				if d.ClassID != "kotlin/collections/CollectionsKt__CollectionsKt" || d.Type == nil || d.Type.ClassID != "java/util/ArrayList" ||
					d.Type.Abbrev == nil || d.Type.Abbrev.ClassID != "kotlin/collections/ArrayList" || len(d.Type.Abbrev.Args) != 1 {
					t.Errorf("arrayListOf: %+v, type %+v", d, d.Type)
				}
			}
		}
		if !found || len(classes) == 0 {
			t.Errorf("%s: arrayListOf %v, %d classes", tc.name, found, len(classes))
		}
	}
	for _, h := range []int64{model, session} {
		w = wire.Writer{}
		w.Int(h)
		if _, err := kcabi.Call(OpKaClose, w.Message()); err != nil {
			t.Fatal(err)
		}
	}
}

// readDecls reads declarations as jvm/kc-aa's KcModel does.
func readDecls(r *wire.Reader) []*ka.Decl {
	var out []*ka.Decl
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		d := &ka.Decl{Kind: ka.Kind(r.Uint()), Name: r.String(), Node: psi.NodeID(r.Uint()), ClassID: r.String(),
			Visibility: ka.Visibility(r.Uint()), Modality: ka.Modality(r.Uint()), Flags: ka.Flags(r.Uint()),
			Variance: ka.Variance(r.Uint())}
		d.Type = readType(r)
		d.Receiver = readType(r)
		d.Supertypes = readTypes(r)
		d.Bounds = readTypes(r)
		for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
			d.Annotations = append(d.Annotations, *readAnnotation(r))
		}
		d.TypeParams = readDecls(r)
		d.Params = readDecls(r)
		d.Members = readDecls(r)
		d.Default = readValue(r)
		d.Locals = readDecls(r)
		d.Module = r.String()
		for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
			d.Inheritors = append(d.Inheritors, r.String())
		}
		out = append(out, d)
	}
	return out
}

func readTypes(r *wire.Reader) []*ka.Type {
	var out []*ka.Type
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		out = append(out, readType(r))
	}
	return out
}

func readType(r *wire.Reader) *ka.Type {
	kind := ka.TypeKind(r.Uint())
	if kind == 0 {
		return nil
	}
	t := &ka.Type{Kind: kind, ClassID: r.String()}
	flags := r.Uint()
	t.Nullable, t.Flexible, t.DefinitelyNonNull, t.Extension, t.Inferred = flags&1 != 0, flags&2 != 0, flags&4 != 0, flags&8 != 0, flags&16 != 0
	t.Param = psi.NodeID(r.Uint())
	t.Name = r.String()
	t.Contexts = int(r.Uint())
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		t.Args = append(t.Args, ka.TypeArg{Variance: ka.Variance(r.Uint()), Type: readType(r)})
	}
	t.Abbrev = readType(r)
	t.Annotations = readAnnotations(r)
	return t
}

func readAnnotations(r *wire.Reader) []ka.Annotation {
	var out []ka.Annotation
	for n := r.Uint(); n > 0 && r.Err() == nil; n-- {
		out = append(out, *readAnnotation(r))
	}
	return out
}
