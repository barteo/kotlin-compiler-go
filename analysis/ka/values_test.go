package ka

import (
	"fmt"
	"strings"
	"testing"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// dumpValue writes v with the text of its nodes: value{text}.
func dumpValue(sb *strings.Builder, snap *psi.Snapshot, v *Value) {
	node := func(id psi.NodeID) {
		if id != 0 {
			fmt.Fprintf(sb, "{%s}", snap.Node(id).Text())
		}
	}
	switch v.Kind {
	case ValueArray:
		sb.WriteString("[")
		for i, e := range v.Elems {
			if i > 0 {
				sb.WriteString(", ")
			}
			dumpValue(sb, snap, e)
		}
		sb.WriteString("]")
	case ValueAnnotation:
		dumpAnnotation(sb, snap, v.Annotation)
	default:
		sb.WriteString(v.String())
	}
	node(v.Node)
}

func dumpAnnotation(sb *strings.Builder, snap *psi.Snapshot, a *Annotation) {
	fmt.Fprintf(sb, "@%s(", a.ClassID)
	for i, arg := range a.Args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.Name + "=")
		dumpValue(sb, snap, arg.Value)
	}
	sb.WriteString(")")
}

func TestAnnotationArgs(t *testing.T) {
	m := update(t, `@file:Suppress("unused", "x")
package p

import kotlin.reflect.KClass

enum class Color { RED, GREEN }

annotation class Inner(val s: String = "in", vararg val xs: Int)

@Retention(AnnotationRetention.SOURCE)
annotation class A(
    val s: String = "dflt",
    val i: Int = 1 + 2,
    val l: Long = 7,
    val c: Char = 'c',
    val z: Boolean = false,
    val f: Float = 1.5f,
    val d: Double = 2.0,
    val u: UInt = 3u,
    val e: Color = Color.GREEN,
    val k: KClass<*> = Any::class,
    val arr: Array<String> = ["x", "y"],
    val inner: Inner = Inner("d", 1, 2),
    vararg val v: String,
)

const val C = "const"

@A(s = C + "!", i = -4, l = 1L shl 40, c = 'q', z = true, f = 0.25f, d = 1e3, u = 0xFFFFFFFFu,
   e = Color.RED, k = String::class, arr = arrayOf("a", C), inner = Inner(xs = [5]))
class X

@A("pos", 9, v = ["p", "q"])
class Y

@A(v = *arrayOf("s1"))
@Inner
class Z

@Missing(1)
@A(i = "wrong", k = Nope::class)
class W

annotation class N(val b: Byte = -2, val s: Short = -3, val i: Int = -6, val l: Long = -7L, val d: Double = -4.0)

@N(b = 1, s = 2, -9, l = 5)
class V
`)
	if len(m.Files) == 0 {
		t.Fatal("no files")
	}
	f := m.Files[0]
	var sb strings.Builder
	for _, a := range f.Annotations {
		sb.WriteString("file: ")
		dumpAnnotation(&sb, f.Snapshot, &a)
		sb.WriteString("\n")
	}
	for _, d := range f.Decls {
		switch d.Name {
		case "X", "Y", "Z", "W", "V":
			fmt.Fprintf(&sb, "%s:", d.Name)
			for _, a := range d.Annotations {
				sb.WriteString(" ")
				dumpAnnotation(&sb, f.Snapshot, &a)
			}
			sb.WriteString("\n")
		case "A", "Inner", "N":
			for _, mem := range d.Members {
				if mem.Kind != KindConstructor {
					continue
				}
				for _, p := range mem.Params {
					if p.Default != nil {
						fmt.Fprintf(&sb, "%s.%s = ", d.Name, p.Name)
						dumpValue(&sb, f.Snapshot, p.Default)
						sb.WriteString("\n")
					}
				}
			}
		}
	}
	want := `file: @kotlin/Suppress(names=["unused"{"unused"}, "x"{"x"}])
Inner.s = "in"{"in"}
A.s = "dflt"{"dflt"}
A.i = 3{1 + 2}
A.l = 7L{7}
A.c = 'c'{'c'}
A.z = false{false}
A.f = 1.5f{1.5f}
A.d = 2.0{2.0}
A.u = 3u{3u}
A.e = p/Color.GREEN{Color.GREEN}
A.k = kotlin/Any::class{Any::class}
A.arr = ["x"{"x"}, "y"{"y"}]{["x", "y"]}
A.inner = @p/Inner(s="d"{"d"}, xs=[1{1}, 2{2}]){Inner("d", 1, 2)}
X: @p/A(s="const!"{C + "!"}, i=-4{-4}, l=1099511627776L{1L shl 40}, c='q'{'q'}, z=true{true}, f=0.25f{0.25f}, d=1000.0{1e3}, u=4294967295u{0xFFFFFFFFu}, e=p/Color.RED{Color.RED}, k=kotlin/String::class{String::class}, arr=["a"{"a"}, "const"{C}]{arrayOf("a", C)}, inner=@p/Inner(xs=[5{5}]{[5]}){Inner(xs = [5])})
Y: @p/A(s="pos"{"pos"}, i=9{9}, v=["p"{"p"}, "q"{"q"}]{["p", "q"]})
Z: @p/A(v=["s1"{"s1"}]{arrayOf("s1")}) @p/Inner()
W: @() @p/A(i=<unsupported>{"wrong"}, k=Nope::class{Nope::class})
N.b = b-2{-2}
N.s = s-3{-3}
N.i = -6{-6}
N.l = -7L{-7L}
N.d = -4.0{-4.0}
V: @p/N(b=b1{1}, s=s2{2}, i=-9{-9}, l=5L{5})
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestJavaClassLiteral checks that a class literal that names a Java class
// kc knows by a Kotlin name keeps the Java name where the file writes
// it, as FIR does: KSP loads the class by that name.
func TestJavaClassLiteral(t *testing.T) {
	m := updateWith(t, withJDK(t), `package p

import kotlin.reflect.KClass
import java.lang.Throwable as JT

annotation class K(vararg val ks: KClass<*>)

@K(JT::class, kotlin.Throwable::class, java.util.Map.Entry::class, Map.Entry::class, java.lang.String::class, String::class)
class C
`)
	var sb strings.Builder
	for _, d := range m.Files[0].Decls {
		if d.Name == "C" {
			for _, a := range d.Annotations {
				dumpAnnotation(&sb, m.Files[0].Snapshot, &a)
			}
		}
	}
	want := `@p/K(ks=[java/lang/Throwable::class{JT::class}, kotlin/Throwable::class{kotlin.Throwable::class}, java/util/Map.Entry::class{java.util.Map.Entry::class}, kotlin/collections/Map.Entry::class{Map.Entry::class}, java/lang/String::class{java.lang.String::class}, kotlin/String::class{String::class}])`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s\ndiagnostics: %v", got, want, m.Diagnostics)
	}
}

func TestJavaNestedDefault(t *testing.T) {
	m := updateWith(t, withJDK(t), `package p

import java.lang.annotation.Retention
import java.lang.annotation.RetentionPolicy
import java.lang.annotation.Target
import java.lang.annotation.ElementType

annotation class K(val r: Retention = Retention(RetentionPolicy.SOURCE), val t: Target = Target(ElementType.FIELD, ElementType.TYPE))
`)
	var sb strings.Builder
	for _, d := range m.Files[0].Decls {
		for _, mem := range d.Members {
			for _, p := range mem.Params {
				if p.Default != nil {
					fmt.Fprintf(&sb, "%s.%s = ", d.Name, p.Name)
					dumpValue(&sb, m.Files[0].Snapshot, p.Default)
					sb.WriteString("\n")
				}
			}
		}
	}
	want := `K.r = @java/lang/annotation/Retention(value=java/lang/annotation/RetentionPolicy.SOURCE{RetentionPolicy.SOURCE}){Retention(RetentionPolicy.SOURCE)}
K.t = @java/lang/annotation/Target(value=[java/lang/annotation/ElementType.FIELD{ElementType.FIELD}, java/lang/annotation/ElementType.TYPE{ElementType.TYPE}]){Target(ElementType.FIELD, ElementType.TYPE)}
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s\ndiagnostics: %v", got, want, m.Diagnostics)
	}
}
