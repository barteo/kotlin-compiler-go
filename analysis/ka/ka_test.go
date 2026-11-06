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
		KindValueParameter: "param", KindTypeParameter: "tparam"}
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
