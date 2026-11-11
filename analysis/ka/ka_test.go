package ka

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func withStdlib(t *testing.T) Config {
	t.Helper()
	jar, err := classpath.FindStdlib()
	if err != nil {
		t.Skipf("no kotlin-stdlib.jar (set KOTLIN_HOME): %v", err)
	}
	return Config{NoJDK: true, ClassPath: []string{jar}}
}

func update(t *testing.T, srcs ...string) *Model {
	t.Helper()
	return updateWith(t, withStdlib(t), srcs...)
}

// withJDK is withStdlib's configuration with the JDK that driver finds,
// as kc finds it by default.
func withJDK(t *testing.T) Config {
	t.Helper()
	cfg := withStdlib(t)
	cfg.NoJDK = false
	if _, err := exec.LookPath("java"); err != nil && os.Getenv("JAVA_HOME") == "" {
		t.Skip("no JDK")
	}
	return cfg
}

func updateWith(t *testing.T, cfg Config, srcs ...string) *Model {
	t.Helper()
	var sources []Source
	for i, s := range srcs {
		sources = append(sources, Source{Path: fmt.Sprintf("f%d.kt", i), Text: &s})
	}
	m, err := Open(cfg).Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// dump writes ds as an indented outline: kind name [classID] node-kind
// @annotations.
func dump(sb *strings.Builder, snap *psi.Snapshot, ds []*Decl, indent string) {
	kinds := map[Kind]string{KindClass: "class", KindInterface: "interface", KindObject: "object",
		KindEnumClass: "enum", KindAnnotationClass: "annotation", KindEnumEntry: "entry",
		KindFunction: "fun", KindProperty: "val", KindTypeAlias: "typealias", KindConstructor: "ctor",
		KindValueParameter: "param", KindTypeParameter: "tparam", KindDestructuring: "destruct"}
	for _, d := range ds {
		node := "-"
		if d.Node != 0 {
			node = snap.Node(d.Node).Kind()
		}
		fmt.Fprintf(sb, "%s%s %s", indent, kinds[d.Kind], d.Name)
		if d.ClassID != "" {
			fmt.Fprintf(sb, " [%s]", d.ClassID)
		}
		fmt.Fprintf(sb, " %s", node)
		for _, a := range d.Annotations {
			fmt.Fprintf(sb, " @%s", a.ClassID)
			if a.Target != "" {
				fmt.Fprintf(sb, "(%s)", a.Target)
			}
			if k := snap.Node(a.Node).Kind(); k != "ANNOTATION_ENTRY" {
				fmt.Fprintf(sb, "!%s", k)
			}
		}
		sb.WriteString("\n")
		dump(sb, snap, d.TypeParams, indent+"  <")
		dump(sb, snap, d.Params, indent+"  (")
		dump(sb, snap, d.Members, indent+"  ")
		dump(sb, snap, d.Locals, indent+"  !")
	}
}

func TestGroupedAnnotations(t *testing.T) {
	m := update(t, `annotation class A1
annotation class A2
@file:[A1 A2]
@[A1 A2]
class C(@param:[A1 A2] val p, @[A1] @A2 val q: Int)
`)
	var sb strings.Builder
	f := m.Files[0]
	dump(&sb, f.Snapshot, f.Decls[2:], "")
	got := sb.String()
	want := `class C [/C] CLASS @A1(file) @A2(file) @A1 @A2
  ctor <init> PRIMARY_CONSTRUCTOR
    (param p VALUE_PARAMETER @A1(param) @A2(param)
    (param q VALUE_PARAMETER @A1 @A2
  val p VALUE_PARAMETER @A1(param) @A2(param)
  val q VALUE_PARAMETER @A1 @A2
`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if len(m.Diagnostics) != 1 || m.Diagnostics[0].Message != "Parameters must have type annotation" {
		t.Errorf("diagnostics: %+v", m.Diagnostics)
	}
}

func TestSetterParamAnnotations(t *testing.T) {
	m := update(t, `annotation class A
class C {
    var e: Int = 0
        set(@setparam:[A] v) { field = v }
    var g: Int = 0
        set(@A v) { field = v }
}
`)
	var sb strings.Builder
	f := m.Files[0]
	dump(&sb, f.Snapshot, f.Decls[1:], "")
	got := sb.String()
	want := `class C [/C] CLASS
  ctor <init> -
  val e PROPERTY @A(setparam)
  val g PROPERTY @A(setparam)
`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestParamFieldAnnotation checks that an annotation without a use-site
// target that may be on a parameter and a field, and not on a property, is
// a constructor property's parameter's only, and a property's field's.
func TestParamFieldAnnotation(t *testing.T) {
	m := update(t, `@Target(AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.FIELD) annotation class R
@Target(AnnotationTarget.FIELD) annotation class F
class C(@R val p: Int, @F val f: Int) {
    @R val q: Int = 0
}
`)
	var sb strings.Builder
	f := m.Files[0]
	dump(&sb, f.Snapshot, f.Decls[2:], "")
	got := sb.String()
	want := `class C [/C] CLASS
  ctor <init> PRIMARY_CONSTRUCTOR
    (param p VALUE_PARAMETER @R
    (param f VALUE_PARAMETER
  val p VALUE_PARAMETER
  val f VALUE_PARAMETER @F(field)
  val q PROPERTY @R(field)
`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestAllTarget checks that an annotation with the use-site target all is
// the property's, its field's, getter's and setter parameter's, and a
// constructor property's parameter's, each where its class may be.
func TestAllTarget(t *testing.T) {
	m := update(t, `annotation class A
@Target(AnnotationTarget.PROPERTY, AnnotationTarget.PROPERTY_GETTER) annotation class G
class C(@all:A var p: Int) {
    @all:A @all:G var q: Int = 0
}
`)
	var sb strings.Builder
	f := m.Files[0]
	dump(&sb, f.Snapshot, f.Decls[2:], "")
	got := sb.String()
	want := `class C [/C] CLASS
  ctor <init> PRIMARY_CONSTRUCTOR
    (param p VALUE_PARAMETER @A(all)
  val p VALUE_PARAMETER @A(all) @A(field) @A(get) @A(setparam)
  val q PROPERTY @A(all) @A(field) @A(get) @A(setparam) @G(all) @G(get)
`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestReceiverAnnotations checks that the annotations on an extension's
// receiver type are the declaration's, with the use-site target receiver.
func TestReceiverAnnotations(t *testing.T) {
	m := update(t, `annotation class A
annotation class B
@receiver:B fun @receiver:A String.f() {}
val @receiver:A String.p: Int get() = 0
`)
	var sb strings.Builder
	f := m.Files[0]
	dump(&sb, f.Snapshot, f.Decls[2:], "")
	got := sb.String()
	want := `fun f FUN @B(receiver) @A(receiver)
val p PROPERTY @A(receiver)
`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestLocals(t *testing.T) {
	m := update(t, `package a
annotation class Anno

class C {
    constructor(x: Int) {
        @Anno val aaa = 2
        println(aaa)
    }
    fun f(): Int {
        var v: String = ""
        @Anno fun g() {}
        @Anno class L
        if (true) { val nested = 1 }
        return 1
    }
    var p: Int
        get() { val inGet = 1; return inGet }
        set(value) { val inSet = value }
}
`)
	var sb strings.Builder
	f := m.Files[0]
	dump(&sb, f.Snapshot, f.Decls, "")
	got := sb.String()
	want := `annotation Anno [a/Anno] CLASS
  ctor <init> -
class C [a/C] CLASS
  ctor <init> SECONDARY_CONSTRUCTOR
    (param x VALUE_PARAMETER
    !val aaa PROPERTY @a/Anno
  fun f FUN
    !val v PROPERTY
    !fun g FUN @a/Anno
    !class L CLASS @a/Anno
    !  ctor <init> -
  val p PROPERTY
    !val inGet PROPERTY
    !val inSet PROPERTY
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, l := range f.Decls[1].Members[1].Locals {
		if l.Visibility != Local || l.ClassID != "" {
			t.Errorf("%s: visibility %d, class ID %q", l.Name, l.Visibility, l.ClassID)
		}
	}
	if v := f.Decls[1].Members[1].Locals[0]; v.Flags&FlagMutable == 0 || v.Type == nil || v.Type.ClassID != "kotlin/String" {
		t.Errorf("v: flags %x, type %+v", v.Flags, v.Type)
	}
}

func TestDecls(t *testing.T) {
	m := update(t, `@file:Suppress("x")
package a.b

@Target(AnnotationTarget.CLASS, AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.FIELD,
    AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.TYPE_PARAMETER)
annotation class Marker
annotation class Other(val s: String)

@Marker
class C<@Marker T>(@Other("p") val x: Int, y: String) {
    @Marker fun f(@Marker z: Int) {}
    @get:Marker val p: Int get() = 1
    constructor() : this(1, "")
    class Nested
    companion object
}

@Marker interface I
enum class E { A, @Marker B }
object O
@Marker fun top() {}
typealias S = String
`)
	f := m.Files[0]
	var sb strings.Builder
	for _, a := range f.Annotations {
		fmt.Fprintf(&sb, "file @%s(%s)\n", a.ClassID, a.Target)
	}
	dump(&sb, f.Snapshot, f.Decls, "")
	want := `file @kotlin/Suppress(file)
annotation Marker [a/b/Marker] CLASS @kotlin/annotation/Target
  ctor <init> -
annotation Other [a/b/Other] CLASS
  ctor <init> PRIMARY_CONSTRUCTOR
    (param s VALUE_PARAMETER
  val s VALUE_PARAMETER
class C [a/b/C] CLASS @a/b/Marker
  <tparam T TYPE_PARAMETER @a/b/Marker
  ctor <init> PRIMARY_CONSTRUCTOR
    (param x VALUE_PARAMETER @a/b/Other
    (param y VALUE_PARAMETER
  val x VALUE_PARAMETER @a/b/Other
  fun f FUN @a/b/Marker
    (param z VALUE_PARAMETER @a/b/Marker
  val p PROPERTY @a/b/Marker(get)
  ctor <init> SECONDARY_CONSTRUCTOR
  class Nested [a/b/C.Nested] CLASS
    ctor <init> -
  object Companion [a/b/C.Companion] OBJECT_DECLARATION
    ctor <init> -
interface I [a/b/I] CLASS @a/b/Marker
enum E [a/b/E] CLASS
  ctor <init> -
  entry A [a/b/E.A] ENUM_ENTRY
  entry B [a/b/E.B] ENUM_ENTRY @a/b/Marker
  fun values -
  fun valueOf -
    (param value -
  val entries -
object O [a/b/O] OBJECT_DECLARATION
  ctor <init> -
fun top FUN @a/b/Marker
typealias S [a/b/S] TYPEALIAS
`
	if got := sb.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestErrors checks that sources with errors still have their facts: a
// syntax error drops only the member it is in, and unresolved names
// leave the rest resolved.
func TestErrors(t *testing.T) {
	m := update(t, `package a
annotation class M
@M class Ok { fun f(): Int = 1 }
`, `package a
@M class Broken {
    @M val x: Strin = 1
    fun g( =
}
@M fun after() {}
`, `package a
@M @Gone class Unresolved : Missing() {
    val y = undefined + 1
    fun h(): Nope = TODO()
}
`)
	var sb strings.Builder
	for _, f := range m.Files {
		dump(&sb, f.Snapshot, f.Decls, f.Path+": ")
	}
	for _, d := range m.Diagnostics {
		fmt.Fprintf(&sb, "%s:%d:%d: %s\n", d.Path, d.Line, d.Col, d.Message)
	}
	want := `f0.kt: annotation M [a/M] CLASS
f0.kt:   ctor <init> -
f0.kt: class Ok [a/Ok] CLASS @a/M
f0.kt:   ctor <init> -
f0.kt:   fun f FUN
f1.kt: class Broken [a/Broken] CLASS @a/M
f1.kt:   ctor <init> -
f1.kt:   val x PROPERTY @a/M
f1.kt: fun after FUN @a/M
f2.kt: class Unresolved [a/Unresolved] CLASS @a/M @
f2.kt:   ctor <init> -
f2.kt:   val y PROPERTY
f2.kt:   fun h FUN
f1.kt:3:15: unresolved or unsupported type: Strin
f1.kt:4:11: Expecting ')'
f2.kt:2:4: unresolved reference: Gone
f2.kt:2:29: unresolved or unsupported type: Missing
f2.kt:3:13: unresolved reference: undefined
f2.kt:4:14: unresolved or unsupported type: Nope
`
	if got := sb.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// typeString writes t as Kotlin writes types, with class IDs, T#node for
// a type parameter and !text! for an error type.
func typeString(snap *psi.Snapshot, t *Type) string {
	if t == nil {
		return "nil"
	}
	var sb strings.Builder
	switch t.Kind {
	case TypeClass:
		if t.Extension {
			sb.WriteString("ext ")
		}
		sb.WriteString(t.ClassID)
		if len(t.Args) > 0 {
			sb.WriteString("<")
			for i, a := range t.Args {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString([...]string{"", "in ", "out ", "*"}[a.Variance])
				if a.Type != nil {
					sb.WriteString(typeString(snap, a.Type))
				}
			}
			sb.WriteString(">")
		}
	case TypeParameter:
		sb.WriteString(t.Name)
		if t.Param == 0 || snap.Node(t.Param).Kind() != "TYPE_PARAMETER" {
			sb.WriteString("#?")
		}
		if t.DefinitelyNonNull {
			sb.WriteString(" & Any")
		}
	case TypeError:
		sb.WriteString("!" + t.Name + "!")
	}
	switch {
	case t.Nullable:
		sb.WriteString("?")
	case t.Flexible:
		sb.WriteString("!")
	}
	return sb.String()
}

func dumpTypes(sb *strings.Builder, snap *psi.Snapshot, ds []*Decl, indent string) {
	for _, d := range ds {
		fmt.Fprintf(sb, "%s%s m%d f%#x", indent, d.Name, d.Modality, d.Flags)
		if d.Variance != Invariant {
			fmt.Fprintf(sb, " v%d", d.Variance)
		}
		if d.Receiver != nil {
			fmt.Fprintf(sb, " recv=%s", typeString(snap, d.Receiver))
		}
		if d.Type != nil {
			fmt.Fprintf(sb, " : %s", typeString(snap, d.Type))
		}
		for _, s := range d.Supertypes {
			fmt.Fprintf(sb, " <: %s", typeString(snap, s))
		}
		for _, s := range d.Bounds {
			fmt.Fprintf(sb, " <: %s", typeString(snap, s))
		}
		sb.WriteString("\n")
		dumpTypes(sb, snap, d.TypeParams, indent+"  <")
		dumpTypes(sb, snap, d.Params, indent+"  (")
		dumpTypes(sb, snap, d.Members, indent+"  ")
	}
}

func TestTypes(t *testing.T) {
	m := updateWith(t, withJDK(t), `package p

abstract class Base<out T : Comparable<T>>(val items: List<T>) : Iterable<T> {
    abstract val first: T
    open fun size(): Int = items.size
    override fun iterator(): Iterator<T> = items.iterator()
    private fun hidden() {}
}

interface Api {
    suspend fun load(id: Long, vararg tags: String): Map<String, Any?>?
    val name: String
    fun greet() = "hi " + name
}

data class Point(val x: Int, var y: Double = 0.0) : Comparable<Point> {
    override fun compareTo(other: Point) = x - other.x
    inline fun <reified R> cast(f: (Point) -> R, g: String.(Int) -> Unit): R = f(this)
    lateinit var label: String
}

sealed interface Shape
enum class Color { RED, GREEN }
annotation class Tag(val value: String)
fun <T> T.orNull(): T? = this
fun <T> nn(t: T & Any): T = t
fun ranges(n: Int) = 1..n
val inferred = listOf(1 to "a")
const val K = 1
fun broken(): Missing = TODO()
fun java(): java.lang.StringBuilder = StringBuilder()
fun props() = System.getProperty("x")
typealias Names = List<String>
`)
	f := m.Files[0]
	var sb strings.Builder
	dumpTypes(&sb, f.Snapshot, f.Decls, "")
	want := `Base m2 f0x0 <: kotlin/collections/Iterable<T>
  <T m0 f0x0 v2 <: kotlin/Comparable<T>
  <init> m0 f0x1000000
    (items m0 f0x0 : kotlin/collections/List<T>
  items m0 f0x2000000 : kotlin/collections/List<T>
  first m2 f0x0 : T
  size m1 f0x0 : kotlin/Int
  iterator m1 f0x100000 : kotlin/collections/Iterator<T>
  hidden m0 f0x0 : kotlin/Unit
Api m2 f0x0 <: kotlin/Any
  load m2 f0x100 : kotlin/collections/Map<kotlin/String, kotlin/Any?>?
    (id m0 f0x0 : kotlin/Long
    (tags m0 f0x10000 : kotlin/String
  name m2 f0x0 : kotlin/String
  greet m1 f0x0 : kotlin/String
Point m0 f0x2 <: kotlin/Comparable<p/Point>
  <init> m0 f0x1000000
    (x m0 f0x0 : kotlin/Int
    (y m0 f0x80000 : kotlin/Double
  x m0 f0x2000000 : kotlin/Int
  y m0 f0x2008000 : kotlin/Double
  compareTo m1 f0x100000 : kotlin/Int
    (other m0 f0x0 : p/Point
  cast m0 f0x80 : R
    <R m0 f0x200000
    (f m0 f0x0 : kotlin/Function1<p/Point, R>
    (g m0 f0x0 : ext kotlin/Function2<kotlin/String, kotlin/Int, kotlin/Unit>
  label m0 f0x200c000 : kotlin/String
  component1 m0 f0x200 : kotlin/Int
  component2 m0 f0x200 : kotlin/Double
  copy m0 f0x0 : p/Point
    (x m0 f0x80000 : kotlin/Int
    (y m0 f0x80000 : kotlin/Double
Shape m3 f0x0 <: kotlin/Any
Color m0 f0x0 <: kotlin/Enum<p/Color>
  <init> m0 f0x1000000
  RED m0 f0x0 : p/Color
  GREEN m0 f0x0 : p/Color
  values m0 f0x800000 : kotlin/Array<p/Color>
  valueOf m0 f0x800000 : p/Color
    (value m0 f0x0 : kotlin/String
  entries m0 f0x800000 : kotlin/enums/EnumEntries<p/Color>
Tag m0 f0x0 <: kotlin/Annotation
  <init> m0 f0x1000000
    (value m0 f0x0 : kotlin/String
  value m0 f0x2000000 : kotlin/String
orNull m0 f0x0 recv=T : T?
  <T m0 f0x0
nn m0 f0x0 : T
  <T m0 f0x0
  (t m0 f0x0 : T & Any
ranges m0 f0x0 : kotlin/ranges/IntRange
  (n m0 f0x0 : kotlin/Int
inferred m0 f0x2000000 : kotlin/collections/List<kotlin/Pair<kotlin/Int, kotlin/String>>
K m0 f0x2002000 : kotlin/Int
broken m0 f0x0 : !Missing!
java m0 f0x0 : java/lang/StringBuilder
props m0 f0x0 : kotlin/String!
Names m0 f0x0 : kotlin/collections/List<kotlin/String>
`
	if got := sb.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestEmpty checks a module without Kotlin sources, as a KSP module with
// only Java sources is.
func TestEmpty(t *testing.T) {
	m := updateWith(t, withStdlib(t))
	defer m.Close()
	if len(m.Files) != 0 {
		t.Errorf("files: %d, want 0", len(m.Files))
	}
	if p := m.Package("kotlin"); p == nil {
		t.Error("package kotlin: nil")
	}
}

// TestBackingField checks which properties have a backing field.
func TestBackingField(t *testing.T) {
	m := update(t, `package p
import kotlin.properties.Delegates
abstract class C(val ctor: Int, plain: Int) {
	val init = 1
	val getter get() = 1
	var usesField = 1
		get() = field + 1
	var customSetter: Int = 0
		set(v) {}
	var customBoth: Int
		get() = 0
		set(v) {}
	val delegated by lazy { 1 }
	var observed: Int by Delegates.notNull()
	abstract val abs: Int
	lateinit var late: String
	val Int.ext get() = this
	val explicit: List<Int>
		field = mutableListOf()
}
interface I {
	val inInterface: Int
	val withGetter get() = 1
}
val topLevel = 1
const val constant = "c"
`)
	defer m.Close()
	var got []string
	var walk func(ds []*Decl)
	walk = func(ds []*Decl) {
		for _, d := range ds {
			if d.Kind == KindProperty {
				got = append(got, fmt.Sprintf("%s=%v", d.Name, d.Flags&FlagBackingField != 0))
			}
			walk(d.Members)
		}
	}
	walk(m.Files[0].Decls)
	for _, id := range []string{"kotlin/Pair", "kotlin/text/Regex", "kotlin/collections/AbstractList"} {
		c := m.Class(id)
		if c == nil {
			t.Fatalf("%s: nil", id)
		}
		for _, d := range c.Members {
			if d.Kind == KindProperty && slices.Contains([]string{"first", "pattern", "options", "size", "modCount"}, d.Name) {
				got = append(got, fmt.Sprintf("%s.%s=%v", id, d.Name, d.Flags&FlagBackingField != 0))
			}
		}
	}
	for _, d := range m.Package("kotlin.collections").Decls {
		if d.Name == "lastIndex" && d.Kind == KindProperty {
			got = append(got, fmt.Sprintf("kotlin.collections.%s=%v", d.Name, d.Flags&FlagBackingField != 0))
			break
		}
	}
	want := []string{
		"ctor=true", "init=true", "getter=false", "usesField=true", "customSetter=true", "customBoth=false",
		"delegated=false", "observed=false", "abs=false", "late=true", "ext=false", "explicit=true",
		"inInterface=false", "withGetter=false", "topLevel=true", "constant=true",
		"kotlin/Pair.first=true", "kotlin/text/Regex.pattern=false", "kotlin/text/Regex.options=false",
		"kotlin/collections/AbstractList.size=false", "kotlin.collections.lastIndex=false",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDestructuring(t *testing.T) {
	m := update(t, `package a
annotation class Anno

fun f() {
    @Anno val (@Anno x, y: Int) = Pair<String, Int>("", 1)
    var (_, @[Anno] z) = 1 to 2L
}
`)
	var sb strings.Builder
	f := m.Files[0]
	dump(&sb, f.Snapshot, f.Decls, "")
	got := sb.String()
	want := `annotation Anno [a/Anno] CLASS
  ctor <init> -
fun f FUN
  !destruct <destruct> DESTRUCTURING_DECLARATION @a/Anno
  !  (val x DESTRUCTURING_DECLARATION_ENTRY @a/Anno
  !  (val y DESTRUCTURING_DECLARATION_ENTRY
  !destruct <destruct> DESTRUCTURING_DECLARATION
  !  (val <unused var> DESTRUCTURING_DECLARATION_ENTRY
  !  (val z DESTRUCTURING_DECLARATION_ENTRY @a/Anno
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	ds := f.Decls[1].Locals
	if len(ds) != 2 || ds[0].Kind != KindDestructuring || ds[0].Visibility != Local {
		t.Fatalf("locals %+v", ds)
	}
	// Bodies aren't checked, so only written entry types are known.
	y, z := ds[0].Params[1], ds[1].Params[1]
	if y.Visibility != Local || y.Type == nil || y.Type.ClassID != "kotlin/Int" || y.Flags&FlagMutable != 0 {
		t.Errorf("y: visibility %d, flags %x, type %+v", y.Visibility, y.Flags, y.Type)
	}
	if z.Visibility != Local || z.Flags&FlagMutable == 0 {
		t.Errorf("z: visibility %d, flags %x", z.Visibility, z.Flags)
	}
}

// TestPartialTypes checks that a class type with an unresolved argument,
// and a generic call with too many type arguments, keep their class, as
// FIR has them, and only the unresolved parts are error types.
func TestPartialTypes(t *testing.T) {
	m := update(t, `package p
class C {
    val inArg: Map<String, Missing> = TODO()
    val nested: List<Map<Missing, Int>?> = TODO()
    val tooMany = mutableMapOf<String, Missing, Int>()
    val whole: Missing = TODO()
}
`)
	f := m.Files[0]
	var sb strings.Builder
	dumpTypes(&sb, f.Snapshot, f.Decls, "")
	want := `C m0 f0x0 <: kotlin/Any
  <init> m0 f0x1000000
  inArg m0 f0x2000000 : kotlin/collections/Map<kotlin/String, !Missing!>
  nested m0 f0x2000000 : kotlin/collections/List<kotlin/collections/Map<!Missing!, kotlin/Int>?>
  tooMany m0 f0x2000000 : kotlin/collections/MutableMap<!!, !!>
  whole m0 f0x2000000 : !Missing!
`
	if got := sb.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestLocalFunReturnType checks that a local function in a body the
// signature check doesn't check has its written return type, as a local
// property has, instead of none (Tachiyomi's AndroidCookieJar.remove,
// whose missing type the kc backend couldn't answer). The checker doesn't
// resolve the types written in such a body, so they are error types named
// by their text.
func TestLocalFunReturnType(t *testing.T) {
	m := update(t, `package a
class C {
    fun f(names: List<String>) {
        fun List<String>.filterNames(): List<String> = this
        fun noType() {}
    }
}
`)
	defer m.Close()
	ls := m.Files[0].Decls[0].Members[1].Locals
	if len(ls) != 2 {
		t.Fatalf("locals: %d", len(ls))
	}
	if r := ls[0].Type; r == nil || r.Kind != TypeError || r.Name != "List<String>" {
		t.Errorf("filterNames: return type %+v", r)
	}
	if r := ls[1].Type; r != nil {
		t.Errorf("noType: return type %+v, want none written", r)
	}
}

// TestEnumEntryBody checks that an enum entry with a body has its
// declarations as members, and FlagEntryBody, and one without has
// neither.
func TestEnumEntryBody(t *testing.T) {
	m := update(t, `package a
enum class Op {
    PLUS {
        override fun apply(a: Int): Int = a
        val extra: String = ""
    },
    MINUS;
    open fun apply(a: Int): Int = -a
}
`)
	defer m.Close()
	var plus, minus *Decl
	for _, d := range m.Files[0].Decls[0].Members {
		if d.Kind == KindEnumEntry {
			switch d.Name {
			case "PLUS":
				plus = d
			case "MINUS":
				minus = d
			}
		}
	}
	if plus == nil || minus == nil {
		t.Fatal("entries missing")
	}
	if plus.Flags&FlagEntryBody == 0 || len(plus.Members) != 2 ||
		plus.Members[0].Name != "apply" || plus.Members[0].Kind != KindFunction || plus.Members[0].Flags&FlagOverride == 0 ||
		plus.Members[1].Name != "extra" || plus.Members[1].Kind != KindProperty {
		t.Errorf("PLUS: flags %x, members %d", plus.Flags, len(plus.Members))
		for _, mm := range plus.Members {
			t.Logf("  %s %d %x", mm.Name, mm.Kind, mm.Flags)
		}
	}
	if minus.Flags&FlagEntryBody != 0 || len(minus.Members) != 0 {
		t.Errorf("MINUS: flags %x, members %d", minus.Flags, len(minus.Members))
	}
}

// TestNullableErrorType checks that an unresolved type written nullable is
// a nullable error type, as in FIR.
func TestNullableErrorType(t *testing.T) {
	m := update(t, `package a
class C {
    fun f(b: Bundle?, c: Bundle) {}
}
`)
	defer m.Close()
	var ps []*Decl
	for _, d := range m.Files[0].Decls[0].Members {
		if d.Name == "f" {
			ps = d.Params
		}
	}
	if len(ps) != 2 {
		t.Fatalf("params: %d", len(ps))
	}
	if b := ps[0].Type; b == nil || b.Kind != TypeError || b.Name != "Bundle" || !b.Nullable {
		t.Errorf("b: %+v", b)
	}
	if c := ps[1].Type; c == nil || c.Kind != TypeError || c.Name != "Bundle" || c.Nullable {
		t.Errorf("c: %+v", c)
	}
}

// TestParameterName checks that a function type's parameter written with
// a name has the annotation @ParameterName with it, as in FIR, and one
// without a name has none.
func TestParameterName(t *testing.T) {
	m := update(t, `package a
var onScale: ((newScale: Float, Int) -> Unit)? = null
`)
	defer m.Close()
	ty := m.Files[0].Decls[0].Type
	if ty == nil || len(ty.Args) != 3 {
		t.Fatalf("type: %+v", ty)
	}
	as := ty.Args[0].Type.Annotations
	if len(as) != 1 || as[0].ClassID != "kotlin/ParameterName" || len(as[0].Args) != 1 ||
		as[0].Args[0].Name != "name" || as[0].Args[0].Value.Str != "newScale" || as[0].Args[0].Value.Node == 0 {
		t.Errorf("newScale: annotations %+v", as)
	}
	if as := ty.Args[1].Type.Annotations; len(as) != 0 {
		t.Errorf("Int: annotations %+v", as)
	}
	if as := ty.Args[2].Type.Annotations; len(as) != 0 {
		t.Errorf("Unit: annotations %+v", as)
	}
}

// TestInvalidFuncType checks that a function type with an unresolved part
// is a function type whose unresolved parts are error types, as in FIR,
// and not an error type.
func TestInvalidFuncType(t *testing.T) {
	m := update(t, `package a
class C {
    var tap: ((MotionEvent) -> Unit)? = null
    fun f(g: Event.(Int) -> Boolean) {}
}
`)
	defer m.Close()
	var tap, g *Type
	for _, d := range m.Files[0].Decls[0].Members {
		switch d.Name {
		case "tap":
			tap = d.Type
		case "f":
			g = d.Params[0].Type
		}
	}
	if tap == nil || tap.Kind != TypeClass || tap.ClassID != "kotlin/Function1" || !tap.Nullable || len(tap.Args) != 2 ||
		tap.Args[0].Type.Kind != TypeError || tap.Args[0].Type.Name != "MotionEvent" ||
		tap.Args[1].Type.Kind != TypeClass || tap.Args[1].Type.ClassID != "kotlin/Unit" {
		t.Errorf("tap: %+v", tap)
	}
	if g == nil || g.Kind != TypeClass || g.ClassID != "kotlin/Function2" || !g.Extension || len(g.Args) != 3 ||
		g.Args[0].Type.Kind != TypeError || g.Args[1].Type.ClassID != "kotlin/Int" || g.Args[2].Type.ClassID != "kotlin/Boolean" {
		t.Errorf("g: %+v", g)
	}
}

// TestErrorTypeArgs checks that, as in FIR, a call whose parameter or
// extension receiver has an unresolved type takes an argument or receiver
// of an unresolved type, so that an initializer's type is still inferred.
func TestErrorTypeArgs(t *testing.T) {
	m := update(t, `package a
class P(c: Context)
fun Context.color(i: Int): Int = i
class D(val context: Context) {
    val p = P(context)
    val q = context.color(1)
}
`)
	defer m.Close()
	got := map[string]string{}
	for _, d := range m.Files[0].Decls[2].Members {
		if d.Type != nil {
			got[d.Name] = d.Type.ClassID
		}
	}
	if got["p"] != "a/P" || got["q"] != "kotlin/Int" {
		t.Errorf("types: %v", got)
	}
}

// TestInferredLocals checks that a body with a local declaration whose
// type isn't written is checked, under the signature check, to infer it,
// as FIR does for KSP's processors (Tachiyomi's destructurings and local
// vals), and that an inferred error type is named as FIR names it: by
// the unresolved callee or reference, or the callee's type parameter
// that wasn't inferred. As in FIR, an error type fits an extension's
// receiver, and a cast is of its type whatever it casts.
func TestInferredLocals(t *testing.T) {
	m := update(t, `package a
fun Int.isDark(): Boolean = this > 0
class C {
    fun f(p: Pair<String, Int>) {
        val (s, i) = p
        val n = s.length
        val u = missing()
        val v = p.nope
        val w = gone.apply { }
        val x = u ?: v
        fun g() = s
        val y = gone.pixel.isDark()
        val z = gone.get() as? C
    }
}
`)
	defer m.Close()
	ls := m.Files[0].Decls[1].Members[1].Locals
	var sb strings.Builder
	for _, l := range ls {
		if l.Kind == KindDestructuring {
			for _, e := range l.Params {
				fmt.Fprintf(&sb, "%s %s\n", e.Name, typeString(nil, e.Type))
			}
			continue
		}
		fmt.Fprintf(&sb, "%s %s\n", l.Name, typeString(nil, l.Type))
	}
	want := `s kotlin/String
i kotlin/Int
n kotlin/Int
u !missing!
v !nope!
w !T!
x !K!
g kotlin/String
y kotlin/Boolean
z a/C?
`
	if got := sb.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
