package ka

import (
	"context"
	"fmt"
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
	var sources []Source
	for i, s := range srcs {
		sources = append(sources, Source{Path: fmt.Sprintf("f%d.kt", i), Text: &s})
	}
	m, err := Open(withStdlib(t)).Update(context.Background(), sources)
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
