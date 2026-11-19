package ka

import (
	"strings"
	"testing"
)

// TestExpect checks that an expect declaration without an actual one is a
// declaration of its file, where it is written and with its members
// expect, that the names of it resolve, and that one with an actual
// declaration is left to that.
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
fun both FUN
`
	if got := sb.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	wantExpects := "DbMaker make Clock <init> zone(field) now tick Companion system Bare clock defaultClock"
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
