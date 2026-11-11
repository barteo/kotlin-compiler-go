package ka

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// dumpLibrary writes the outline of library declaration d: kind name
// flags, its annotations, types and parameters' defaults.
func dumpLibrary(sb *strings.Builder, d *Decl, indent string, filter func(*Decl) bool) {
	fmt.Fprintf(sb, "%s%d %s", indent, d.Kind, d.Name)
	if d.ClassID != "" {
		fmt.Fprintf(sb, " [%s]", d.ClassID)
	}
	fmt.Fprintf(sb, " v%d m%d f%#x", d.Visibility, d.Modality, d.Flags)
	for _, a := range d.Annotations {
		sb.WriteString(" " + a.String())
	}
	for _, tp := range d.TypeParams {
		fmt.Fprintf(sb, " <%s%s", [...]string{"", "in ", "out ", "*"}[tp.Variance], tp.Name)
		for _, b := range tp.Bounds {
			sb.WriteString(" : " + libType(b))
		}
		sb.WriteString(">")
	}
	if d.Receiver != nil {
		sb.WriteString(" recv " + libType(d.Receiver))
	}
	if len(d.Params) > 0 {
		sb.WriteString(" (")
		for i, p := range d.Params {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%s: %s", p.Name, libType(p.Type))
			for _, a := range p.Annotations {
				sb.WriteString(" " + a.String())
			}
			if p.Default != nil {
				sb.WriteString(" = " + p.Default.String())
			} else if p.Flags&FlagHasDefault != 0 {
				sb.WriteString(" = ...")
			}
		}
		sb.WriteString(")")
	}
	if d.Type != nil {
		sb.WriteString(": " + libType(d.Type))
	}
	if len(d.Supertypes) > 0 {
		sb.WriteString(" :")
		for _, st := range d.Supertypes {
			sb.WriteString(" " + libType(st))
		}
	}
	sb.WriteString("\n")
	for _, m := range d.Members {
		if filter == nil || filter(m) {
			dumpLibrary(sb, m, indent+"  ", filter)
		}
	}
}

func TestLibraryClass(t *testing.T) {
	m := update(t, "package p\nclass A")
	defer m.Close()
	if d := m.Class("p/A"); d != nil {
		t.Errorf("p/A is a source class, got %+v", d)
	}
	if d := m.Class("no/Such"); d != nil {
		t.Errorf("no/Such: got %+v", d)
	}
	var sb strings.Builder
	for _, id := range []string{"kotlin/Deprecated", "kotlin/DeprecationLevel", "kotlin/ReplaceWith"} {
		d := m.Class(id)
		if d == nil {
			t.Fatalf("%s: nil", id)
		}
		dumpLibrary(&sb, d, "", nil)
	}
	d := m.Class("kotlin/collections/AbstractList")
	if d == nil {
		t.Fatal("AbstractList: nil")
	}
	dumpLibrary(&sb, d, "", func(m *Decl) bool { return m.Name == "get" || m.Name == "<init>" || m.Name == "size" })
	d = m.Class("kotlin/coroutines/AbstractCoroutineContextKey")
	if d == nil {
		t.Fatal("AbstractCoroutineContextKey: nil")
	}
	dumpLibrary(&sb, d, "", nil)
	want := `5 Deprecated [kotlin/Deprecated] v0 m0 f0x0 @kotlin/annotation/Target(allowedTargets = [kotlin/annotation/AnnotationTarget.CLASS, kotlin/annotation/AnnotationTarget.FUNCTION, kotlin/annotation/AnnotationTarget.PROPERTY, kotlin/annotation/AnnotationTarget.ANNOTATION_CLASS, kotlin/annotation/AnnotationTarget.CONSTRUCTOR, kotlin/annotation/AnnotationTarget.PROPERTY_SETTER, kotlin/annotation/AnnotationTarget.PROPERTY_GETTER, kotlin/annotation/AnnotationTarget.TYPEALIAS]) @kotlin/annotation/MustBeDocumented() : kotlin/Annotation
  10 <init> v0 m0 f0x1000000 (message: kotlin/String, replaceWith: kotlin/ReplaceWith = @kotlin/ReplaceWith(expression = "", imports = []), level: kotlin/DeprecationLevel = kotlin/DeprecationLevel.WARNING)
  8 message v0 m0 f0x0: kotlin/String
  8 replaceWith v0 m0 f0x0: kotlin/ReplaceWith
  8 level v0 m0 f0x0: kotlin/DeprecationLevel
4 DeprecationLevel [kotlin/DeprecationLevel] v0 m0 f0x0 : kotlin/Enum<kotlin/DeprecationLevel>
  10 <init> v1 m0 f0x1000000
  6 WARNING [kotlin/DeprecationLevel.WARNING] v0 m0 f0x0: kotlin/DeprecationLevel
  6 ERROR [kotlin/DeprecationLevel.ERROR] v0 m0 f0x0: kotlin/DeprecationLevel
  6 HIDDEN [kotlin/DeprecationLevel.HIDDEN] v0 m0 f0x0: kotlin/DeprecationLevel
  7 values v0 m0 f0x800000: kotlin/Array<kotlin/DeprecationLevel>
  7 valueOf v0 m0 f0x800000 (value: kotlin/String): kotlin/DeprecationLevel
  8 entries v0 m0 f0x800000: kotlin/enums/EnumEntries<kotlin/DeprecationLevel>
5 ReplaceWith [kotlin/ReplaceWith] v0 m0 f0x0 @kotlin/annotation/Target(allowedTargets = []) @kotlin/annotation/Retention(value = kotlin/annotation/AnnotationRetention.BINARY) @kotlin/annotation/MustBeDocumented() : kotlin/Annotation
  10 <init> v0 m0 f0x1000000 (expression: kotlin/String, imports: kotlin/String)
  8 expression v0 m0 f0x0: kotlin/String
  8 imports v0 m0 f0x0: kotlin/Array<out kotlin/String>
1 AbstractList [kotlin/collections/AbstractList] v0 m2 f0x0 @kotlin/SinceKotlin(version = "1.1") <out E> : kotlin/collections/AbstractCollection<E> kotlin/collections/List<E>
  10 <init> v2 m0 f0x1000000
  8 size v0 m2 f0x0: kotlin/Int
  7 get v0 m2 f0x200 (index: kotlin/Int): E
1 AbstractCoroutineContextKey [kotlin/coroutines/AbstractCoroutineContextKey] v0 m2 f0x0 @kotlin/SinceKotlin(version = "1.3") @kotlin/ExperimentalStdlibApi() @kotlin/Deprecated(message = "Polymorphic coroutine context keys are error-prone, difficult to implement correctly, and can encourage depending on implementation details. Prefer retrieving the element by its base key and casting it explicitly when needed or introducing a dedicated extension property.") @kotlin/DeprecatedSinceKotlin(warningSince = "2.4") <B : kotlin/coroutines/CoroutineContext.Element> <E : B> : kotlin/coroutines/CoroutineContext.Key<E>
  10 <init> v0 m0 f0x1000000 (baseKey: kotlin/coroutines/CoroutineContext.Key<B>, safeCast: kotlin/Function1<kotlin/coroutines/CoroutineContext.Element, E?>)
  8 safeCast v1 m0 f0x2000000: kotlin/Function1<kotlin/coroutines/CoroutineContext.Element, E?>
  8 topmostKey v1 m0 f0x2000000: kotlin/coroutines/CoroutineContext.Key<*>
  7 tryCast v3 m0 f0x0 (element: kotlin/coroutines/CoroutineContext.Element): E?
  7 isSubKey v3 m0 f0x0 (key: kotlin/coroutines/CoroutineContext.Key<*>): kotlin/Boolean
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if m.Class("kotlin/Deprecated") != m.Class("kotlin/Deprecated") {
		t.Error("Class isn't cached")
	}
}

func TestLibraryJavaClass(t *testing.T) {
	m := updateWith(t, withJDK(t), "package p\nclass A")
	defer m.Close()
	var sb strings.Builder
	for _, id := range []string{"java/lang/annotation/Retention", "java/lang/Thread.State", "java/util/AbstractMap.SimpleEntry"} {
		d := m.Class(id)
		if d == nil {
			t.Fatalf("%s: nil", id)
		}
		dumpLibrary(&sb, d, "", func(m *Decl) bool {
			return m.Kind != KindFunction || m.Name == "getKey" || m.Name == "setValue" || m.Name == "value"
		})
	}
	want := `5 Retention [java/lang/annotation/Retention] v0 m2 f0x400000 @java/lang/annotation/Documented() @java/lang/annotation/Retention(value = java/lang/annotation/RetentionPolicy.RUNTIME) @java/lang/annotation/Target(value = [java/lang/annotation/ElementType.ANNOTATION_TYPE]) : kotlin/Annotation
  10 <init> v0 m0 f0x1400000 (value: java/lang/annotation/RetentionPolicy)
  8 value v0 m0 f0x400000: java/lang/annotation/RetentionPolicy
4 State [java/lang/Thread.State] v0 m0 f0xc00000 : kotlin/Enum<java/lang/Thread.State!>
  6 NEW [java/lang/Thread.State.NEW] v0 m0 f0x0: java/lang/Thread.State
  6 RUNNABLE [java/lang/Thread.State.RUNNABLE] v0 m0 f0x0: java/lang/Thread.State
  6 BLOCKED [java/lang/Thread.State.BLOCKED] v0 m0 f0x0: java/lang/Thread.State
  6 WAITING [java/lang/Thread.State.WAITING] v0 m0 f0x0: java/lang/Thread.State
  6 TIMED_WAITING [java/lang/Thread.State.TIMED_WAITING] v0 m0 f0x0: java/lang/Thread.State
  6 TERMINATED [java/lang/Thread.State.TERMINATED] v0 m0 f0x0: java/lang/Thread.State
  10 <init> v1 m0 f0x400000
  8 entries v0 m0 f0xc00000: kotlin/enums/EnumEntries<java/lang/Thread.State>
1 SimpleEntry [java/util/AbstractMap.SimpleEntry] v0 m1 f0xc00000 <K : kotlin/Any!> <V : kotlin/Any!> : kotlin/collections/MutableMap.MutableEntry<K!, V!> java/io/Serializable
  13 key v1 m0 f0x400000: K!
  13 value v1 m0 f0x408000: V!
  7 getKey v0 m1 f0x400000: K!
  7 setValue v0 m1 f0x400000 (p0: V!): V!
  10 <init> v0 m0 f0x400000 (p0: K!, p1: V!)
  10 <init> v0 m0 f0x400000 (p0: kotlin/collections/MutableMap.MutableEntry<out K!, out V!>!)
  13 serialVersionUID v1 m0 f0xc00000: kotlin/Long
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// libType writes t with its arguments.
func libType(t *Type) string {
	if t == nil {
		return "<nil>"
	}
	var sb strings.Builder
	switch t.Kind {
	case TypeClass:
		sb.WriteString(t.ClassID)
	case TypeParameter:
		sb.WriteString(t.Name)
	default:
		sb.WriteString("<error " + t.Name + ">")
	}
	if len(t.Args) > 0 {
		sb.WriteString("<")
		for i, a := range t.Args {
			if i > 0 {
				sb.WriteString(", ")
			}
			switch a.Variance {
			case Star:
				sb.WriteString("*")
				continue
			case In:
				sb.WriteString("in ")
			case Out:
				sb.WriteString("out ")
			}
			sb.WriteString(libType(a.Type))
		}
		sb.WriteString(">")
	}
	if t.Nullable {
		sb.WriteString("?")
	}
	if t.Flexible {
		sb.WriteString("!")
	}
	if t.Abbrev != nil {
		sb.WriteString(" (as " + libType(t.Abbrev) + ")")
	}
	return sb.String()
}

func TestLibraryBuiltins(t *testing.T) {
	m := updateWith(t, withJDK(t), "package p\nclass A")
	defer m.Close()
	var sb strings.Builder
	for _, id := range []string{"kotlin/Any", "kotlin/String", "kotlin/Int", "kotlin/collections/List",
		"kotlin/collections/Map", "kotlin/Comparable", "kotlin/Function1"} {
		d := m.Class(id)
		if d == nil {
			t.Fatalf("%s: nil", id)
		}
		// Their members come from .kotlin_builtins, and the collections'
		// JDK ones from their Java classes; a few of String's and Int's.
		some := map[string]bool{"length": true, "get": true, "toLong": true, "inc": true, "Companion": true, "MAX_VALUE": true}
		filter := func(m *Decl) bool { return id != "kotlin/String" && id != "kotlin/Int" || some[m.Name] }
		dumpLibrary(&sb, d, "", filter)
	}
	want := `1 Any [kotlin/Any] v0 m1 f0x0
  10 <init> v0 m0 f0x1000000
  7 equals v0 m1 f0x200 (other: kotlin/Any?): kotlin/Boolean
  7 hashCode v0 m1 f0x0: kotlin/Int
  7 toString v0 m1 f0x0: kotlin/String
1 String [kotlin/String] v0 m0 f0x0 : java/io/Serializable kotlin/Comparable<kotlin/String> kotlin/CharSequence
  8 length v0 m1 f0x0: kotlin/Int
  7 get v0 m1 f0x200 (index: kotlin/Int): kotlin/Char
  3 Companion [kotlin/String.Companion] v0 m0 f0x800001 : kotlin/Any
1 Int [kotlin/Int] v0 m0 f0x0 : kotlin/Number kotlin/Comparable<kotlin/Int>
  7 inc v0 m0 f0x200: kotlin/Int
  7 toLong v0 m1 f0x0: kotlin/Long
  3 Companion [kotlin/Int.Companion] v0 m0 f0x800001 : kotlin/Any
    8 MAX_VALUE v0 m0 f0x2000: kotlin/Int
2 List [kotlin/collections/List] v0 m2 f0x0 <out E> : kotlin/collections/Collection<E>
  8 size v0 m2 f0x0: kotlin/Int
  7 isEmpty v0 m2 f0x0: kotlin/Boolean
  7 contains v0 m2 f0x200 (element: E): kotlin/Boolean
  7 iterator v0 m2 f0x200: kotlin/collections/Iterator<E>
  7 containsAll v0 m2 f0x0 (elements: kotlin/collections/Collection<E>): kotlin/Boolean
  7 get v0 m2 f0x200 (index: kotlin/Int): E
  7 indexOf v0 m2 f0x0 (element: E): kotlin/Int
  7 lastIndexOf v0 m2 f0x0 (element: E): kotlin/Int
  7 listIterator v0 m2 f0x0: kotlin/collections/ListIterator<E>
  7 listIterator v0 m2 f0x0 (index: kotlin/Int): kotlin/collections/ListIterator<E>
  7 subList v0 m2 f0x0 (fromIndex: kotlin/Int, toIndex: kotlin/Int): kotlin/collections/List<E>
  7 spliterator v0 m1 f0x400000: java/util/Spliterator<E!>!
  7 getFirst v0 m1 f0x400000: E!
  7 getLast v0 m1 f0x400000: E!
2 Map [kotlin/collections/Map] v0 m2 f0x0 <K> <out V> : kotlin/Any
  8 size v0 m2 f0x0: kotlin/Int
  8 keys v0 m2 f0x0: kotlin/collections/Set<K>
  8 values v0 m2 f0x0: kotlin/collections/Collection<V>
  8 entries v0 m2 f0x0: kotlin/collections/Set<kotlin/collections/Map.Entry<K, V>>
  7 isEmpty v0 m2 f0x0: kotlin/Boolean
  7 containsKey v0 m2 f0x0 (key: K): kotlin/Boolean
  7 containsValue v0 m2 f0x0 (value: V): kotlin/Boolean
  7 get v0 m2 f0x200 (key: K): V?
  7 getOrDefault v0 m1 f0x0 (key: K, defaultValue: V): V
  7 forEach v0 m1 f0x400000 (p0: java/util/function/BiConsumer<in K!, in V!>!): kotlin/Unit
  2 Entry [kotlin/collections/Map.Entry] v0 m2 f0x800000 <out K> <out V> : kotlin/Any
    8 key v0 m2 f0x0: K
    8 value v0 m2 f0x0: V
2 Comparable [kotlin/Comparable] v0 m2 f0x0 <in T> : kotlin/Any
  7 compareTo v0 m2 f0x200 (other: T): kotlin/Int
2 Function1 [kotlin/Function1] v0 m2 f0x0 <in P1> <out R> : kotlin/Function<R>
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJavaSources(t *testing.T) {
	java := `package j;

import java.util.List;

public class JavaSrc<T> extends Base implements Runnable {
    public static final int MAX = 3;
    protected List<String> names;
    NonExist broken;
    Missing gone(Missing m) { return m; }
    @Deprecated public JavaSrc(String first, int count) {}
    public void run() {}
    public <R> R map(T value, R fallback) { return fallback; }
    static class Nested { void inside(long l) {} }
    public enum Mode { ON, OFF }
}

class Base {}
class Orphan extends Gone implements Runnable { public void run() {} }
abstract class Mixed implements Runnable, Lost, Comparable<String> {}
`
	kotlin := "package j\nclass UsesJava(val j: JavaSrc<String>) : Base()\nfun f() = JavaSrc.MAX\n"
	sources := []Source{{Path: "j/Uses.kt", Text: &kotlin}, {Path: "j/JavaSrc.java", Text: &java}}
	s := Open(withJDK(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, d := range m.Diagnostics {
		t.Errorf("diagnostic: %+v", d)
	}
	if len(m.Files) != 1 || m.Files[0].Path != "j/Uses.kt" {
		t.Errorf("files: want the Kotlin one, got %d", len(m.Files))
	}
	d := m.Class("j/JavaSrc")
	if d == nil {
		t.Fatal("j/JavaSrc: nil")
	}
	var sb strings.Builder
	dumpLibrary(&sb, d, "", func(m *Decl) bool { return m.Kind != KindFunction || !strings.HasPrefix(m.Name, "<") })
	want := `1 JavaSrc [j/JavaSrc] v0 m1 f0x4400000 <T : kotlin/Any!> : j/Base java/lang/Runnable
  13 names v2 m0 f0x408000: kotlin/collections/MutableList<kotlin/String!>!
  13 broken v4 m0 f0x408000: <error NonExist>!
  7 gone v4 m1 f0x400000 (m: <error Missing>!): <error Missing>!
  7 run v0 m1 f0x400000: kotlin/Unit
  7 map v0 m1 f0x400000 <R : kotlin/Any!> (value: T!, fallback: R!): R!
  10 <init> v0 m0 f0x400000 @java/lang/Deprecated() (first: kotlin/String!, count: kotlin/Int)
  13 MAX v0 m0 f0xc00000: kotlin/Int
  1 Nested [j/JavaSrc.Nested] v4 m1 f0x4c00000 : kotlin/Any
    7 inside v4 m1 f0x400000 (l: kotlin/Long): kotlin/Unit
    10 <init> v4 m0 f0x400000
  4 Mode [j/JavaSrc.Mode] v0 m0 f0x4c00000 : kotlin/Enum<j/JavaSrc.Mode!>
    6 ON [j/JavaSrc.Mode.ON] v0 m0 f0x0: j/JavaSrc.Mode
    6 OFF [j/JavaSrc.Mode.OFF] v0 m0 f0x0: j/JavaSrc.Mode
    7 values v0 m0 f0xc00000: kotlin/Array<j/JavaSrc.Mode>
    7 valueOf v0 m0 f0xc00000 (value: kotlin/String): j/JavaSrc.Mode
    8 entries v0 m0 f0xc00000: kotlin/enums/EnumEntries<j/JavaSrc.Mode>
`
	if got := sb.String(); got != want {
		t.Errorf("j/JavaSrc:\n%s\nwant:\n%s", got, want)
	}
	if d := m.Class("j/Base"); d == nil || d.Flags&FlagJavaSource == 0 {
		t.Errorf("j/Base: want a Java source class, got %+v", d)
	}
	if d := m.Class("j/Orphan"); d == nil || len(d.Supertypes) != 2 || d.Supertypes[0].Kind != TypeError || d.Supertypes[0].Name != "Gone" ||
		d.Supertypes[1].ClassID != "java/lang/Runnable" {
		t.Errorf("j/Orphan: want supertypes <error Gone>, Runnable, got %+v", d)
	}
	if d := m.Class("j/Mixed"); d == nil || len(d.Supertypes) != 3 || d.Supertypes[0].ClassID != "java/lang/Runnable" ||
		d.Supertypes[1].Kind != TypeError || d.Supertypes[1].Name != "Lost" || d.Supertypes[2].ClassID != "kotlin/Comparable" {
		t.Errorf("j/Mixed: want supertypes Runnable, <error Lost>, Comparable, got %+v", d)
	}
	if d := m.Class("java/lang/Thread"); d == nil || d.Flags&FlagJavaSource != 0 {
		t.Errorf("java/lang/Thread: want a JDK class, got %+v", d)
	}
}

func TestLibraryConstants(t *testing.T) {
	m := update(t, "package p\nconst val K = 1 + 2\nconst val S = \"a\" + K\nval notConst = 3\n")
	defer m.Close()
	find := func(ds []*Decl, name string) *Decl {
		for _, d := range ds {
			if d.Name == name {
				return d
			}
		}
		t.Fatalf("no %s", name)
		return nil
	}
	for _, c := range []struct {
		d    *Decl
		want string
	}{
		{find(m.Package("kotlin.io").Decls, "DEFAULT_BUFFER_SIZE"), "Int 8192"},
		{find(m.Class("kotlin/KotlinVersion.Companion").Members, "MAX_COMPONENT_VALUE"), "Int 255"},
		{find(m.Class("kotlin/text/Typography").Members, "quote"), "Char 34"},
		{find(m.Files[0].Decls, "K"), "Int 3"},
		{find(m.Files[0].Decls, "S"), "String a3"},
	} {
		v := c.d.Default
		if v == nil || v.Kind != ValueConst {
			t.Errorf("%s: %+v, want a constant", c.d.Name, v)
			continue
		}
		got := fmt.Sprintf("%v %d", map[ConstKind]string{ConstInt: "Int", ConstChar: "Char"}[v.Const], v.Bits)
		if v.Const == ConstString {
			got = "String " + v.Str
		}
		if got != c.want {
			t.Errorf("%s: %s, want %s", c.d.Name, got, c.want)
		}
	}
	if d := find(m.Files[0].Decls, "notConst"); d.Default != nil {
		t.Errorf("notConst: %+v, want no value", d.Default)
	}
}

// TestInvisibleJavaClass checks that a Kotlin class keeps the type of a
// package-private Java class of another package, reported as invisible, as
// FIR does, and that a Java class keeps unresolved supertypes in place.
func TestInvisibleJavaClass(t *testing.T) {
	i0 := "package com.example\ninterface I0\n"
	i1 := "package com.example;\ninterface I1 {}\n"
	k := "import com.example.*\nclass K : I0, I1, I2\n"
	j := "import com.example.*;\nclass J implements I0, I1, I2 {}\n"
	s := Open(withJDK(t))
	defer s.Close()
	m, err := s.Update(context.Background(), []Source{{Path: "K.kt", Text: &k}, {Path: "J.java", Text: &j},
		{Path: "com/example/I0.kt", Text: &i0}, {Path: "com/example/I1.java", Text: &i1}})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	str := func(ts []*Type) string {
		var out []string
		for _, t := range ts {
			if t.Kind == TypeError {
				out = append(out, "<error "+t.Name+">")
			} else {
				out = append(out, t.ClassID)
			}
		}
		return strings.Join(out, ", ")
	}
	want := "com/example/I0, com/example/I1, <error I2>"
	if d := m.Files[0].Decls[0]; d.Name != "K" || str(d.Supertypes) != want {
		t.Errorf("K: want %s, got %s %s", want, d.Name, str(d.Supertypes))
	}
	if d := m.Class("J"); d == nil || str(d.Supertypes) != want {
		t.Errorf("J: want %s, got %+v", want, d)
	}
}

func TestJavaNullability(t *testing.T) {
	notNull := "package org.jetbrains.annotations;\npublic @interface NotNull {}\n"
	nullable := "package org.jetbrains.annotations;\npublic @interface Nullable {}\n"
	java := `package j;

import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

public class NN {
    @Nullable @NotNull public String both;
    @Nullable public String nullable;
    @NotNull public String notNull;
    public String none;
    public int prim;
    @NotNull public String get(@Nullable String p, String q) { return ""; }
}
`
	kotlin := "package j\nval a = NN().notNull\nval b = NN().nullable\nval c = NN().none\nval d = NN().get(null, null)\n"
	sources := []Source{{Path: "j/a.kt", Text: &kotlin}, {Path: "j/NN.java", Text: &java},
		{Path: "org/jetbrains/annotations/NotNull.java", Text: &notNull}, {Path: "org/jetbrains/annotations/Nullable.java", Text: &nullable}}
	s := Open(withJDK(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, d := range m.Diagnostics {
		t.Errorf("diagnostic: %+v", d)
	}
	d := m.Class("j/NN")
	if d == nil {
		t.Fatal("j/NN: nil")
	}
	var sb strings.Builder
	dumpLibrary(&sb, d, "", func(m *Decl) bool { return m.Kind != KindFunction || !strings.HasPrefix(m.Name, "<") })
	sb.WriteString("--\n")
	dumpTypes(&sb, m.Files[0].Snapshot, m.Files[0].Decls, "")
	want := `1 NN [j/NN] v0 m1 f0x4400000 : kotlin/Any
  13 both v0 m0 f0x408000 @org/jetbrains/annotations/Nullable() @org/jetbrains/annotations/NotNull(): kotlin/String!
  13 nullable v0 m0 f0x408000 @org/jetbrains/annotations/Nullable(): kotlin/String?
  13 notNull v0 m0 f0x408000 @org/jetbrains/annotations/NotNull(): kotlin/String
  13 none v0 m0 f0x408000: kotlin/String!
  13 prim v0 m0 f0x408000: kotlin/Int
  7 get v0 m1 f0x400000 @org/jetbrains/annotations/NotNull() (p: kotlin/String? @org/jetbrains/annotations/Nullable(), q: kotlin/String!): kotlin/String
  10 <init> v0 m0 f0x400000
--
a m0 f0x2000000 : kotlin/String
b m0 f0x2000000 : kotlin/String?
c m0 f0x2000000 : kotlin/String!
d m0 f0x2000000 : kotlin/String
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestJavaInnerClass checks that a Java inner class reads its outer class's
// type parameters, and that a J<String>.H type keeps its outer arguments.
func TestJavaInnerClass(t *testing.T) {
	java := `package j;

import java.util.List;

public class Base<T, P> {
    public class Inner<P> {
        public T t;
        public List<T> list;
        public Base<T, P> base;
        public T get(P p) { return null; }
    }
    public static class Nested<Q> {
        public Q q;
    }
}
`
	k := `package j;

public class K {
    public Base<String, Integer>.Inner<Long> x;
    public Base.Nested<String> y;
}
`
	kotlin := "package j\nval a = K().x.t\nval b = K().x.get(1L)\nval c = K().y.q\n"
	sources := []Source{{Path: "j/a.kt", Text: &kotlin}, {Path: "j/Base.java", Text: &java}, {Path: "j/K.java", Text: &k}}
	s := Open(withJDK(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, d := range m.Diagnostics {
		t.Errorf("diagnostic: %+v", d)
	}
	var sb strings.Builder
	for _, name := range []string{"j/Base.Inner", "j/K"} {
		d := m.Class(name)
		if d == nil {
			t.Fatalf("%s: nil", name)
		}
		dumpLibrary(&sb, d, "", func(m *Decl) bool { return m.Kind != KindFunction || !strings.HasPrefix(m.Name, "<") })
	}
	sb.WriteString("--\n")
	dumpTypes(&sb, m.Files[0].Snapshot, m.Files[0].Decls, "")
	// x's arguments are kc's, the inner class's own first, then its outer
	// class's; the JVM side splits them per qualifier.
	want := `1 Inner [j/Base.Inner] v0 m1 f0x4400004 <P : kotlin/Any!> : kotlin/Any
  13 t v0 m0 f0x408000: T!
  13 list v0 m0 f0x408000: kotlin/collections/MutableList<T!>!
  13 base v0 m0 f0x408000: j/Base<T!, P!>!
  7 get v0 m1 f0x400000 (p: P!): T!
  10 <init> v0 m0 f0x400000
1 K [j/K] v0 m1 f0x4400000 : kotlin/Any
  13 x v0 m0 f0x408000: j/Base.Inner<kotlin/Long!, kotlin/String!, kotlin/Int!>!
  13 y v0 m0 f0x408000: j/Base.Nested<kotlin/String!>!
  10 <init> v0 m0 f0x400000
--
a m0 f0x2000000 : kotlin/String!
b m0 f0x2000000 : kotlin/String!
c m0 f0x2000000 : kotlin/String!
`
	if got := sb.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestAnnotatedJavaCollection checks that a Java collection type with a
// nullability annotation is still flexible between the mutable and the
// read-only collection, as FIR's (Mutable)List<String>? is.
func TestAnnotatedJavaCollection(t *testing.T) {
	java := `package j;

import java.util.List;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

public class Coll {
    public @Nullable List<String> maybe() { return null; }
    public @NotNull List<String> sure() { return null; }
    public List<String> plain() { return null; }
    public @Nullable String text() { return null; }
}
`
	nullable := "package org.jetbrains.annotations;\npublic @interface Nullable {}\n"
	notNull := "package org.jetbrains.annotations;\npublic @interface NotNull {}\n"
	kotlin := "package j\nfun f(c: Coll) = c.maybe()\n"
	sources := []Source{{Path: "j/Uses.kt", Text: &kotlin}, {Path: "j/Coll.java", Text: &java},
		{Path: "org/jetbrains/annotations/Nullable.java", Text: &nullable},
		{Path: "org/jetbrains/annotations/NotNull.java", Text: &notNull}}
	s := Open(withJDK(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	d := m.Class("j/Coll")
	if d == nil {
		t.Fatal("j/Coll: nil")
	}
	want := map[string][3]bool{ // Nullable, Flexible, DefinitelyNonNull
		"maybe": {true, true, false},
		"sure":  {false, true, true},
		"plain": {false, true, false},
		"text":  {true, false, false},
	}
	for _, md := range d.Members {
		w, ok := want[md.Name]
		if !ok {
			continue
		}
		delete(want, md.Name)
		ty := md.Type
		if ty == nil {
			t.Errorf("%s: no type", md.Name)
			continue
		}
		if got := [3]bool{ty.Nullable, ty.Flexible, ty.DefinitelyNonNull}; got != w {
			t.Errorf("%s: %s nullable, flexible, definitely non-null %v, want %v", md.Name, ty.ClassID, got, w)
		}
	}
	for n := range want {
		t.Errorf("%s: missing", n)
	}
}

// TestMissingClassParam checks that a Java constructor or method whose
// signature names a class not on the class path (as android.jar's are
// in KSP's Tachiyomi benchmark) can still be called, with an error type
// for that class, as FIR calls it: the types of declarations that call
// it are inferred.
func TestMissingClassParam(t *testing.T) {
	java := `package j;

public class Prefs {
    public Prefs(Missing m) {}
    public static Prefs of(Missing m) { return null; }
    public Gone gone() { return null; }
}
`
	kotlin := `package j
class Uses(m: Missing) {
    val a = Prefs(m)
    val b = Prefs.of(m)
    val c = a.gone()
}
`
	sources := []Source{{Path: "j/Uses.kt", Text: &kotlin}, {Path: "j/Prefs.java", Text: &java}}
	s := Open(withJDK(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range m.Files[0].Decls[0].Members {
		if d.Type != nil {
			got[d.Name] = d.Type.ClassID
			if d.Type.Kind == TypeError {
				got[d.Name] = "error"
			}
		}
	}
	if got["a"] != "j/Prefs" || got["b"] != "j/Prefs" || got["c"] != "error" {
		t.Errorf("types: %v", got)
	}
}
