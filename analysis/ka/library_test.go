package ka

import (
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
		fmt.Fprintf(sb, " <%s", tp.Name)
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
5 ReplaceWith [kotlin/ReplaceWith] v0 m0 f0x0 @kotlin/annotation/Target(allowedTargets = []) @kotlin/annotation/Retention(value = kotlin/annotation/AnnotationRetention.BINARY) @kotlin/annotation/MustBeDocumented() : kotlin/Annotation
  10 <init> v0 m0 f0x1000000 (expression: kotlin/String, imports: kotlin/String)
  8 expression v0 m0 f0x0: kotlin/String
  8 imports v0 m0 f0x0: kotlin/Array<out kotlin/String>
1 AbstractList [kotlin/collections/AbstractList] v0 m2 f0x0 @kotlin/SinceKotlin(version = "1.1") <E> : kotlin/collections/AbstractCollection<E> kotlin/collections/List<E>
  10 <init> v2 m0 f0x1000000
  8 size v0 m2 f0x0: kotlin/Int
  7 get v0 m2 f0x200 (index: kotlin/Int): E
1 AbstractCoroutineContextKey [kotlin/coroutines/AbstractCoroutineContextKey] v0 m2 f0x0 @kotlin/SinceKotlin(version = "1.3") @kotlin/ExperimentalStdlibApi() @kotlin/Deprecated(message = "Polymorphic coroutine context keys are error-prone, difficult to implement correctly, and can encourage depending on implementation details. Prefer retrieving the element by its base key and casting it explicitly when needed or introducing a dedicated extension property.") @kotlin/DeprecatedSinceKotlin(warningSince = "2.4") <B : kotlin/coroutines/CoroutineContext.Element> <E : B> : kotlin/coroutines/CoroutineContext.Key<E>
  10 <init> v0 m0 f0x1000000 (baseKey: kotlin/coroutines/CoroutineContext.Key<B>, safeCast: kotlin/Function1<kotlin/coroutines/CoroutineContext.Element, E?>)
  8 safeCast v1 m0 f0x0: kotlin/Function1<kotlin/coroutines/CoroutineContext.Element, E?>
  8 topmostKey v1 m0 f0x0: kotlin/coroutines/CoroutineContext.Key<*>
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
1 SimpleEntry [java/util/AbstractMap.SimpleEntry] v0 m1 f0xc00000 <K> <V> : kotlin/collections/MutableMap.MutableEntry<K!, V!> java/io/Serializable
  10 <init> v0 m0 f0x400000 (p0: K!, p1: V!)
  10 <init> v0 m0 f0x400000 (p0: kotlin/collections/MutableMap.MutableEntry<out K!, out V!>!)
  7 getKey v0 m1 f0x400000: K!
  7 setValue v0 m1 f0x400000 (p0: V!): V!
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
	return sb.String()
}
