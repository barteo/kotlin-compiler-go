package ka

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestExpect checks that an expect declaration without an actual one is a
// declaration of its file, where it is written and with its members
// expect, that the names of it resolve, and that one with an actual
// declaration is a declaration next to it.
func TestExpect(t *testing.T) {
	m := update(t, `package a

interface Maker<T> { fun make(): T }
annotation class By(val k: kotlin.reflect.KClass<*>)

@By(DbMaker::class)
abstract class Db

expect object DbMaker : Maker<Db> {
    override fun make(): Db
}

expect class Clock(val zone: String) {
    val now: Long
    fun tick()
    companion object { fun system(): Clock }
}
expect class Bare
expect fun clock(zone: String = "UTC"): Clock
val user: Clock get() = clock()
expect val defaultClock: Clock
expect val String.width: Int
expect fun both(): Int
`, `package a

actual fun both(): Int = 1
`)
	var sb strings.Builder
	var expects []string
	var walk func(ds []*Decl)
	walk = func(ds []*Decl) {
		for _, d := range ds {
			if d.Flags&FlagExpect != 0 {
				name := d.Name
				if d.Flags&FlagBackingField != 0 {
					name += "(field)"
				}
				expects = append(expects, name)
			}
			walk(d.Members)
		}
	}
	for _, f := range m.Files {
		dump(&sb, f.Snapshot, f.Decls, "")
		walk(f.Decls)
	}
	want := `interface Maker [a/Maker] CLASS
  <tparam T TYPE_PARAMETER
  fun make FUN
annotation By [a/By] CLASS
  ctor <init> PRIMARY_CONSTRUCTOR
    (param k VALUE_PARAMETER
  val k VALUE_PARAMETER
class Db [a/Db] CLASS @a/By
  ctor <init> -
object DbMaker [a/DbMaker] OBJECT_DECLARATION
  fun make FUN
class Clock [a/Clock] CLASS
  ctor <init> PRIMARY_CONSTRUCTOR
    (param zone VALUE_PARAMETER
  val zone VALUE_PARAMETER
  val now PROPERTY
  fun tick FUN
  object Companion [a/Clock.Companion] OBJECT_DECLARATION
    fun system FUN
class Bare [a/Bare] CLASS
fun clock FUN
  (param zone VALUE_PARAMETER
val user PROPERTY
val defaultClock PROPERTY
val width PROPERTY
fun both FUN
fun both FUN
`
	if got := sb.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	wantExpects := "DbMaker make Clock <init> zone(field) now tick Companion system Bare clock defaultClock width both"
	if got := strings.Join(expects, " "); got != wantExpects {
		t.Errorf("expect declarations: %s, want %s", got, wantExpects)
	}
	if len(m.Diagnostics) != 0 {
		t.Errorf("diagnostics: %+v", m.Diagnostics)
	}
	args := m.Files[0].Decls[2].Annotations[0].Args
	if len(args) != 1 || args[0].Value.Type == nil || args[0].Value.Type.ClassID != "a/DbMaker" {
		t.Errorf("@By's argument: %+v", args)
	}
}

// TestExpectActual checks the model of expect declarations whose actual
// ones are in the sources: both are declarations, in the places they are
// written, an expect function or property is matched to the actual one of
// its signature, an expect class is what its name resolves to where the
// actual declaration is a typealias, and an actual declaration's
// parameters have no default values of their own.
func TestExpectActual(t *testing.T) {
	srcs := []string{`package a

expect class Box(n: Int = 1) {
    val n: Int
    val String.width: Int
    fun get(i: Int = 0): Text
}
expect class Text
expect enum class Mode { ON, OFF }
expect fun over(a: Int): Text
expect fun over(a: String): Int
expect fun String.over(a: Int): Int
expect fun <T> over(a: T, b: List<T>): T
expect fun over(a: Long, b: Int): Int
expect val size: Int
expect val String.size: Int
expect val Int.size: Int
expect val lazily: Box
class Use(val box: Box, val text: Text)
fun count(): Int { return 1 }
`, `package a

actual class Box actual constructor(n: Int) {
    actual val n: Int = n
    actual val String.width: Int get() = length
    actual fun get(i: Int): Text = Text()
    fun more() {}
}
actual typealias Text = StringBuilder
actual enum class Mode { ON, OFF }
actual fun over(a: Int): Text = Text()
actual fun String.over(a: Int): Int = a
actual fun <U> over(a: U, b: List<U>): U = a
actual val size: Int = 1
actual val String.size: Int get() = length
actual val lazily: Box by lazy { Box(1) }
fun made() = Box(1).get()
`}
	want := []string{`Box f0x20 [a/Box]
  <init> f0x1000020
    (n f0x80000 : kotlin/Int
  n f0x20 : kotlin/Int
  width f0x20 recv=kotlin/String : kotlin/Int
  get f0x20 : a/Text
    (i f0x80000 : kotlin/Int
Text f0x20 [a/Text]
Mode f0x20 [a/Mode]
  ON : a/Mode
  OFF : a/Mode
  values f0x800020 : kotlin/Array<a/Mode>
  valueOf f0x800020 : a/Mode
    (value : kotlin/String
  entries f0x800020 : kotlin/enums/EnumEntries<a/Mode>
over f0x20 : a/Text
  (a : kotlin/Int
over f0x20 : kotlin/Int
  (a : kotlin/String
over f0x20 recv=kotlin/String : kotlin/Int
  (a : kotlin/Int
over f0x20 : T
  <T
  (a : T
  (b : kotlin/collections/List<T>
over f0x20 : kotlin/Int
  (a : kotlin/Long
  (b : kotlin/Int
size f0x20 : kotlin/Int
size f0x20 recv=kotlin/String : kotlin/Int
size f0x20 recv=kotlin/Int : kotlin/Int
lazily f0x20 : a/Box
Use [a/Use]
  <init> f0x1000000
    (box : a/Box
    (text : a/Text
  box f0x2000000 : a/Box
  text f0x2000000 : a/Text
count : kotlin/Int
`, `Box f0x40 [a/Box]
  <init> f0x1000040
    (n : kotlin/Int
  n f0x2000040 : kotlin/Int
  width f0x40 recv=kotlin/String : kotlin/Int
  get f0x40 : a/Text
    (i : kotlin/Int
  more : kotlin/Unit
Text f0x40 [a/Text] : java/lang/StringBuilder
Mode f0x40 [a/Mode]
  <init> f0x1000000
  ON : a/Mode
  OFF : a/Mode
  values f0x800000 : kotlin/Array<a/Mode>
  valueOf f0x800000 : a/Mode
    (value : kotlin/String
  entries f0x800000 : kotlin/enums/EnumEntries<a/Mode>
over f0x40 : a/Text
  (a : kotlin/Int
over f0x40 recv=kotlin/String : kotlin/Int
  (a : kotlin/Int
over f0x40 : U
  <U
  (a : U
  (b : kotlin/collections/List<U>
size f0x2000040 : kotlin/Int
size f0x40 recv=kotlin/String : kotlin/Int
lazily f0x40 : a/Box
made : a/Text
`}
	var outline func(sb *strings.Builder, f *File, ds []*Decl, indent string)
	outline = func(sb *strings.Builder, f *File, ds []*Decl, indent string) {
		for _, d := range ds {
			sb.WriteString(indent + d.Name)
			if d.Flags != 0 {
				fmt.Fprintf(sb, " f%#x", d.Flags)
			}
			if d.ClassID != "" && d.Kind != KindEnumEntry {
				fmt.Fprintf(sb, " [%s]", d.ClassID)
			}
			if d.Receiver != nil {
				fmt.Fprintf(sb, " recv=%s", typeString(f.Snapshot, d.Receiver))
			}
			if d.Type != nil {
				fmt.Fprintf(sb, " : %s", typeString(f.Snapshot, d.Type))
			}
			sb.WriteString("\n")
			outline(sb, f, d.TypeParams, indent+"  <")
			outline(sb, f, d.Params, indent+"  (")
			outline(sb, f, d.Members, indent+"  ")
		}
	}
	check := func(m *Model) {
		t.Helper()
		for i, f := range m.Files {
			var sb strings.Builder
			outline(&sb, f, f.Decls, "")
			if got := sb.String(); got != want[i] {
				t.Errorf("file %d: got\n%s\nwant\n%s", i, got, want[i])
			}
		}
		if len(m.Diagnostics) != 0 {
			t.Errorf("diagnostics: %+v", m.Diagnostics)
		}
	}
	s := Open(withStdlib(t))
	update := func() *Model {
		t.Helper()
		sources := make([]Source, len(srcs))
		for i := range srcs {
			sources[i] = Source{Path: fmt.Sprintf("f%d.kt", i), Text: &srcs[i]}
		}
		m, err := s.Update(context.Background(), sources)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	check(update())
	// An update that checks the changed file alone has its expect
	// declarations too.
	srcs[0] = strings.Replace(srcs[0], "return 1", "return 2", 1)
	incrementalUpdates.Store(0)
	check(update())
	if incrementalUpdates.Load() != 1 {
		t.Errorf("not checked incrementally")
	}
}

// TestEnumEntryInitializer checks that an enum entry has the declarations
// of its initializer, its body's and a constructor, where FIR gives it
// one: an entry with arguments, annotations or a body, and any entry of an
// enum class that can't be constructed without arguments.
func TestEnumEntryInitializer(t *testing.T) {
	m := update(t, `package a

annotation class Mark
enum class Level(val rank: Int) {
    LOW(1),
    HIGH(2) {
        override fun label() = "high"
        val extra = 1
    };
    open fun label() = "low"
}
enum class Step { ONE, TWO(), THREE {}, @Mark FOUR }
enum class Only { A; constructor(n: Int) }
enum class Either { A; constructor(n: Int); constructor() }
enum class Defaulted(val n: Int = 0) { A }
`)
	var got []string
	for _, d := range m.Files[0].Decls {
		for _, e := range d.Members {
			if e.Kind != KindEnumEntry {
				continue
			}
			s := d.Name + "." + e.Name
			for _, em := range e.Members {
				s += " " + em.Name
			}
			got = append(got, s)
		}
	}
	want := []string{"Level.LOW <init>", "Level.HIGH label extra <init>", "Step.ONE", "Step.TWO <init>", "Step.THREE <init>",
		"Step.FOUR <init>", "Only.A <init>", "Either.A", "Defaulted.A <init>"}
	if !slices.Equal(got, want) {
		t.Errorf("entries: %q, want %q", got, want)
	}
}

// TestExpectActualFacts checks that an OpenFacts session, which checks
// the bodies too, checks an actual declaration's body against the
// typealias that is its expect class's actual declaration: a
// java.lang.Runnable's run, which the expect class doesn't declare, as
// kotlinx-coroutines' Runnable is.
func TestExpectActualFacts(t *testing.T) {
	srcs := []string{`package a

expect interface Runnable
expect class Text()
fun common(r: Runnable, t: Text): Text = t
`, `package a

actual typealias Runnable = java.lang.Runnable
actual typealias Text = StringBuilder
fun jvm(r: Runnable, t: Text): Int {
    r.run()
    return t.append("x").length
}
`}
	sources := make([]Source, len(srcs))
	for i := range srcs {
		sources[i] = Source{Path: fmt.Sprintf("f%d.kt", i), Text: &srcs[i]}
	}
	s := OpenFacts(withJDK(t))
	defer s.Close()
	m, err := s.Update(context.Background(), sources)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if len(m.Diagnostics) != 0 {
		t.Errorf("diagnostics: %+v", m.Diagnostics)
	}
}
