package kmeta

import (
	"archive/zip"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

func stdlib(t *testing.T) string {
	t.Helper()
	path, err := classpath.FindStdlib()
	if err != nil {
		t.Skip(err)
	}
	return path
}

// stdlibClasses parses every class file in the standard library.
func stdlibClasses(t *testing.T) map[string]*classfile.Info {
	t.Helper()
	r, err := zip.OpenReader(stdlib(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	classes := map[string]*classfile.Info{}
	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".class") || strings.HasPrefix(f.Name, "META-INF/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		info, err := classfile.Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		classes[info.Name] = info
	}
	return classes
}

var local = regexp.MustCompile(`\$[0-9]`)

func hasMethod(info *classfile.Info, sig JVMSig) bool {
	return slices.ContainsFunc(info.Methods, func(m classfile.Member) bool {
		return m.Name == sig.Name && m.Desc == sig.Desc
	})
}

// TestStdlibSignatures decodes the metadata of every class in the standard
// library and checks that the JVM signature of each function, constructor
// and property accessor names a method of the class file.
func TestStdlibSignatures(t *testing.T) {
	classes := stdlibClasses(t)
	var nclasses, nfuncs, missing int
	check := func(info *classfile.Info, what string, sig JVMSig) {
		nfuncs++
		if !hasMethod(info, sig) {
			missing++
			if missing <= 20 {
				t.Errorf("%s: %s %s%s not found", info.Name, what, sig.Name, sig.Desc)
			}
		}
	}
	// Every type parameter a signature uses must be in scope.
	var unbound int
	var inner bool
	var walk func(where string, scope []*TypeParam, typ *Type)
	walk = func(where string, scope []*TypeParam, typ *Type) {
		if typ == nil {
			return
		}
		if typ.Class == "" && !slices.ContainsFunc(scope, func(p *TypeParam) bool { return p.ID == typ.Param }) {
			unbound++
			if unbound <= 5 {
				t.Errorf("%s: type parameter %d (%q) not in scope", where, typ.Param, typ.ParamName)
			}
		}
		for _, a := range typ.Args {
			walk(where, scope, a.Type)
		}
		walk(where, scope, typ.Upper)
	}
	checkScope := func(where string, outer, own []*TypeParam, ts ...*Type) {
		// Local and anonymous classes may use the type parameters of the
		// function that declares them, and inner classes those of the
		// outer class.
		if local.MatchString(where) || inner {
			return
		}
		scope := append(slices.Clip(outer), own...)
		for _, typ := range ts {
			walk(where, scope, typ)
		}
		for _, p := range own {
			for _, b := range p.Bounds {
				walk(where, scope, b)
			}
		}
	}
	checkProp := func(info *classfile.Info, p *Property) {
		if p.Getter != nil {
			check(info, "getter", *p.Getter)
		}
		if p.Setter != nil {
			check(info, "setter", *p.Setter)
		}
	}
	for _, info := range classes {
		m := info.Kotlin
		if m == nil {
			continue
		}
		switch m.K {
		case KindClassFile:
			c, err := ReadClass(m)
			if err != nil {
				t.Fatalf("%s: %v", info.Name, err)
			}
			nclasses++
			// Anonymous objects copied from inline functions keep the
			// original's metadata.
			if want := strings.ReplaceAll(c.Name, ".", "$"); want != info.Name && !strings.Contains(info.Name, "$$inlined") {
				t.Errorf("%s: metadata names class %s", info.Name, c.Name)
			}
			inner = c.IsInner()
			checkScope(c.Name, nil, c.TypeParams, c.Supers...)
			for _, f := range c.Funcs {
				check(info, "function", f.JVM)
				checkScope(c.Name+"."+f.Name, c.TypeParams, f.TypeParams, append(paramTypes(f.Params), f.Recv, f.Ret)...)
			}
			// Annotation classes have constructors only in Kotlin.
			for _, k := range c.Ctors {
				if c.Kind() != KindAnnotation {
					check(info, "constructor", k.JVM)
				}
			}
			for _, k := range c.Ctors {
				checkScope(c.Name+".<init>", c.TypeParams, nil, paramTypes(k.Params)...)
			}
			for _, p := range c.Props {
				checkProp(info, p)
				checkScope(c.Name+"."+p.Name, c.TypeParams, p.TypeParams, p.Recv, p.Type)
			}
		case KindFileFacade, KindMultiFilePart:
			inner = false
			p, err := ReadPackage(m)
			if err != nil {
				t.Fatalf("%s: %v", info.Name, err)
			}
			nclasses++
			funcs, props, _ := p.Names()
			for _, name := range funcs {
				fs, err := p.Functions(name)
				if err != nil {
					t.Fatalf("%s.%s: %v", info.Name, name, err)
				}
				for _, f := range fs {
					check(info, "function", f.JVM)
					checkScope(info.Name+"."+f.Name, nil, f.TypeParams, append(paramTypes(f.Params), f.Recv, f.Ret)...)
				}
			}
			for _, name := range props {
				ps, err := p.Properties(name)
				if err != nil {
					t.Fatalf("%s.%s: %v", info.Name, name, err)
				}
				for _, p := range ps {
					checkProp(info, p)
					checkScope(info.Name+"."+p.Name, nil, p.TypeParams, p.Recv, p.Type)
				}
			}
		}
	}
	t.Logf("%d classes, %d members, %d not found, %d unbound type parameters", nclasses, nfuncs, missing, unbound)
	if nclasses < 500 {
		t.Errorf("decoded %d classes, want at least 500", nclasses)
	}
}

func openStdlib(t *testing.T) *classpath.Path {
	t.Helper()
	p, err := classpath.Open("", []string{stdlib(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func loadPackage(t *testing.T, cp *classpath.Path, class string) *Package {
	t.Helper()
	info, err := cp.Load(class)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ReadPackage(info.Kotlin)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFunctions(t *testing.T) {
	cp := openStdlib(t)

	console := loadPackage(t, cp, "kotlin/io/ConsoleKt")
	fs, err := console.Functions("println")
	if err != nil {
		t.Fatal(err)
	}
	var descs []string
	for _, f := range fs {
		descs = append(descs, f.JVM.Desc)
	}
	if !slices.Contains(descs, "(Ljava/lang/Object;)V") || !slices.Contains(descs, "(I)V") || !slices.Contains(descs, "()V") {
		t.Errorf("println overloads: %v", descs)
	}

	part := loadPackage(t, cp, "kotlin/collections/CollectionsKt___CollectionsKt")
	fs, err = part.Functions("map")
	if err != nil {
		t.Fatal(err)
	}
	var m *Function
	for _, f := range fs {
		if f.Recv != nil && f.Recv.Class == "kotlin/collections/Iterable" {
			m = f
		}
	}
	if m == nil {
		t.Fatal("Iterable<T>.map not found")
	}
	if !m.IsInline() || len(m.TypeParams) != 2 || len(m.Params) != 1 {
		t.Errorf("map: inline %v, %d type params, %d params", m.IsInline(), len(m.TypeParams), len(m.Params))
	}
	tp := m.TypeParams[0]
	if m.Recv.Args[0].Type.Param != tp.ID || tp.Name != "T" {
		t.Errorf("map's receiver is not Iterable<T>: %+v", m.Recv.Args[0].Type)
	}
	if fn := m.Params[0].Type; fn.Class != "kotlin/Function1" || len(fn.Args) != 2 || fn.Args[0].Proj != Inv || fn.ExtFunc {
		t.Errorf("map's parameter: %+v", fn)
	}

	// apply's block is a function type with a receiver: T.() -> Unit.
	fs, err = loadPackage(t, cp, "kotlin/StandardKt__StandardKt").Functions("apply")
	if err != nil || len(fs) != 1 {
		t.Fatalf("apply: %v, %d functions", err, len(fs))
	}
	if fn := fs[0].Params[0].Type; fn.Class != "kotlin/Function1" || !fn.ExtFunc {
		t.Errorf("apply's parameter: %+v", fn)
	}
	if m.Ret.Class != "kotlin/collections/List" || m.Ret.Nullable {
		t.Errorf("map returns %+v", m.Ret)
	}
	if m.JVM != (JVMSig{"map", "(Ljava/lang/Iterable;Lkotlin/jvm/functions/Function1;)Ljava/util/List;"}) {
		t.Errorf("map's JVM signature: %v", m.JVM)
	}

	fs, err = part.Functions("joinToString")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.Params[0].Name != "separator" || f.Params[0].Flags&ParamDeclaresDefault == 0 {
			t.Errorf("joinToString's separator: %+v", f.Params[0])
		}
	}

	fs, err = loadPackage(t, cp, "kotlin/collections/CollectionsKt__CollectionsKt").Functions("listOf")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(fs, func(f *Function) bool {
		return len(f.Params) == 1 && f.Params[0].Vararg != nil && f.Params[0].Type.Class == "kotlin/Array"
	}) {
		t.Error("listOf(vararg elements: T) not found")
	}
}

func TestModule(t *testing.T) {
	cp := openStdlib(t)
	data, err := cp.Resource("META-INF/kotlin-stdlib.kotlin_module")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadModule(data)
	if err != nil {
		t.Fatal(err)
	}
	parts := m.Packages["kotlin.collections"]
	if !slices.Contains(parts, Part{"kotlin/collections/CollectionsKt___CollectionsKt", "kotlin/collections/CollectionsKt"}) {
		t.Errorf("kotlin.collections parts: %v", parts)
	}
	if !slices.Contains(m.Packages["kotlin.io"], Part{"kotlin/io/ConsoleKt", ""}) {
		t.Errorf("kotlin.io parts: %v", m.Packages["kotlin.io"])
	}
	for pkg, parts := range m.Packages {
		for _, p := range parts {
			if !cp.Has(p.Class) || p.Facade != "" && !cp.Has(p.Facade) {
				t.Errorf("%s: part %v not on the class path", pkg, p)
			}
		}
	}

	// kotlin.collections.ArrayList is a typealias in a file facade.
	var found bool
	for _, p := range parts {
		info, err := cp.Load(p.Class)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := ReadPackage(info.Kotlin)
		if err != nil {
			t.Fatal(err)
		}
		as, err := pkg.TypeAliases("ArrayList")
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range as {
			found = a.Expanded.Class == "java/util/ArrayList" && len(a.TypeParams) == 1
		}
	}
	if !found {
		t.Error("typealias ArrayList<E> = java.util.ArrayList<E> not found")
	}
}

func TestWriteModule(t *testing.T) {
	data := WriteModule(map[string][]ModulePart{"a.b": {{Name: "FKt"}, {Name: "EKt"}, {Name: "G__HKt", Facade: "G"}}, "": {{Name: "MainKt"}}})
	m, err := ReadModule(data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]Part{"a.b": {{"a/b/G__HKt", "a/b/G"}, {"a/b/EKt", ""}, {"a/b/FKt", ""}}, "": {{"MainKt", ""}}}
	if !reflect.DeepEqual(m.Packages, want) {
		t.Errorf("got %v, want %v", m.Packages, want)
	}
}

func TestBuiltins(t *testing.T) {
	cp := openStdlib(t)
	names := cp.Resources(".kotlin_builtins")
	if !slices.Contains(names, "kotlin/collections/collections.kotlin_builtins") {
		t.Fatalf("builtins files: %v", names)
	}
	classes := map[string]*Class{}
	for _, n := range names {
		data, err := cp.Resource(n)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ReadBuiltins(data)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		for _, c := range b.Classes {
			classes[c.Name] = c
		}
	}
	list := classes["kotlin/collections/List"]
	if list == nil {
		t.Fatalf("no kotlin.collections.List among %d classes", len(classes))
	}
	if list.Kind() != KindInterface || len(list.TypeParams) != 1 || list.TypeParams[0].Variance != Out {
		t.Errorf("List: kind %v, type params %+v", list.Kind(), list.TypeParams)
	}
	if len(list.Supers) != 1 || list.Supers[0].Class != "kotlin/collections/Collection" {
		t.Errorf("List's supertypes: %+v", list.Supers)
	}
	if !slices.ContainsFunc(list.Funcs, func(f *Function) bool {
		return f.Name == "get" && len(f.Params) == 1 && f.Params[0].Type.Class == "kotlin/Int" &&
			f.Ret.Param == list.TypeParams[0].ID && f.IsOperator()
	}) {
		t.Error("operator fun get(index: Int): E not found")
	}
	if !slices.ContainsFunc(list.Props, func(p *Property) bool { return p.Name == "size" }) {
		t.Error("List.size not found")
	}
	if c := classes["kotlin/collections/Map.Entry"]; c == nil || len(c.TypeParams) != 2 {
		t.Errorf("Map.Entry: %+v", c)
	}
	if c := classes["kotlin/Int"]; c == nil || !slices.ContainsFunc(c.Funcs, func(f *Function) bool { return f.Name == "toLong" }) {
		t.Error("Int.toLong not found")
	}
}

func TestConcurrentDecode(t *testing.T) {
	cp := openStdlib(t)
	p := loadPackage(t, cp, "kotlin/collections/CollectionsKt___CollectionsKt")
	funcs, _, _ := p.Names()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for _, name := range funcs {
				if _, err := p.Functions(name); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
}

// TestRoundTrip decodes the metadata of every class and facade in the
// standard library, encodes it again, and checks that it decodes to the
// same declarations.
func TestRoundTrip(t *testing.T) {
	classes := stdlibClasses(t)
	var n, failed int
	meta := func(h Header) *classfile.KotlinMeta {
		return &classfile.KotlinMeta{K: h.K, MV: h.MV, XI: h.XI, D1: h.D1, D2: h.D2}
	}
	report := func(name string, a, b any) {
		failed++
		if failed <= 10 {
			t.Errorf("%s: decoded again differently:\n%s", name, firstDiff(reflect.ValueOf(a), reflect.ValueOf(b), ""))
		}
	}
	for name, info := range classes {
		m := info.Kotlin
		if m == nil {
			continue
		}
		switch m.K {
		case KindClassFile:
			c, err := ReadClass(m)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			c2, err := ReadClass(meta(WriteClass(c)))
			if err != nil {
				t.Fatalf("%s: encoded: %v", name, err)
			}
			n++
			if !reflect.DeepEqual(c, c2) {
				report(name, c, c2)
			}
		case KindFileFacade, KindMultiFilePart:
			p, err := ReadPackage(m)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			d, err := p.All()
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			p2, err := ReadPackage(meta(WritePackage(d)))
			if err != nil {
				t.Fatalf("%s: encoded: %v", name, err)
			}
			d2, err := p2.All()
			if err != nil {
				t.Fatalf("%s: encoded: %v", name, err)
			}
			if slices.ContainsFunc(d.Funcs, func(f *Function) bool { return f.Contexts > len(f.ContextParams) }) {
				continue // context receivers, which kc doesn't encode
			}
			n++
			if !reflect.DeepEqual(d, d2) {
				report(name, d, d2)
			}
		}
	}
	t.Logf("%d classes and facades encoded again, %d differ", n, failed)
}

// firstDiff describes the first difference between two values.
func firstDiff(a, b reflect.Value, path string) string {
	if a.Kind() != b.Kind() {
		return path + ": kinds differ"
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				return path + ": nil differs"
			}
			return ""
		}
		return firstDiff(a.Elem(), b.Elem(), path)
	case reflect.Struct:
		for i := range a.NumField() {
			if d := firstDiff(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name); d != "" {
				return d
			}
		}
		return ""
	case reflect.Slice:
		if a.Len() != b.Len() {
			return path + ": lengths " + strconv.Itoa(a.Len()) + " and " + strconv.Itoa(b.Len())
		}
		for i := range a.Len() {
			if d := firstDiff(a.Index(i), b.Index(i), path+"["+strconv.Itoa(i)+"]"); d != "" {
				return d
			}
		}
		return ""
	}
	if a.CanInterface() && !reflect.DeepEqual(a.Interface(), b.Interface()) {
		return path + ": " + fmt.Sprint(a.Interface()) + " != " + fmt.Sprint(b.Interface())
	}
	return ""
}

// TestEncodeD1 checks that long metadata is split into strings that a
// class file's Utf8 constants can hold, and decodes back.
func TestEncodeD1(t *testing.T) {
	b := make([]byte, 100000)
	for i := range b {
		b[i] = byte(i * 7) // NULs and bytes above 0x7f take two bytes each
	}
	d1 := encodeD1(b)
	if len(d1) < 2 {
		t.Fatalf("%d strings, want the bytes split", len(d1))
	}
	c := classfile.NewClass(0, "T", "java/lang/Object")
	for _, s := range d1 {
		c.Utf8(s) // panics if the modified UTF-8 is over 65535 bytes
	}
	if got := Bytes(d1); !slices.Equal(got, b) {
		t.Errorf("decoded %d bytes, want the %d encoded", len(got), len(b))
	}
}
