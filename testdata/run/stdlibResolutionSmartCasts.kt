// Name resolution, smart casts and data flow that kotlinc accepts.

import kotlin.concurrent.atomics.AtomicInt
import kotlin.concurrent.atomics.AtomicReference
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.reflect.KProperty

// A smart cast from an `as` in a when entry's first condition.
fun firstEntryCast(x: Any): Int {
    when (x as String) {
        "a" -> return x.length
    }
    return x.length + 10
}

// A smart cast of a stable top-level property, named through its package.
val stable: Any = "pkg"

fun packageQualified(): Int {
    if (stable is String) return stable.length
    return 0
}

// Unary plus on a literal takes the expected type.
val b: Byte = +5
val sh: Short = +7

// A branch ending in a call of an inline function returning Nothing jumps.
inline fun fail(): Nothing = throw IllegalStateException("no")

fun inlinedJump(x: String?): Int {
    if (x == null) {
        fail()
    }
    return x.length
}

fun whenJump(x: String?): Int {
    when {
        x == null -> fail()
        else -> {}
    }
    return x.length
}

// A delegated property's setter.
class Box {
    var stored = ""
    operator fun getValue(r: Any?, p: KProperty<*>) = stored
    operator fun setValue(r: Any?, p: KProperty<*>, v: String) {
        stored = v
    }
}

class Holder {
    var name: String by Box()
}

// A typealias of String, nested or not, called as a constructor.
typealias Text = String

class Aliases {
    typealias Nested = String
}

// An annotated lambda after explicit type arguments.
annotation class Marked

fun <T> make(f: () -> T): T = f()

@OptIn(ExperimentalAtomicApi::class)
fun atomics(): String {
    val n = AtomicInt(1)
    n.store(5)
    val prev = n.fetchAndAdd(2)
    val r = AtomicReference("x")
    r.store("y")
    return "$prev ${n.load()} ${r.exchange("z")} ${r.load()} ${n.addAndFetch(1)}"
}

fun main() {
    println(firstEntryCast("a"))
    println(firstEntryCast("abc"))
    println(packageQualified())
    println(b + sh)
    println(inlinedJump("four"))
    println(whenJump("fives"))
    val h = Holder()
    h.name = "set"
    println(h.name)
    println("[" + Text() + Aliases.Nested() + "]")
    println(make<Int> @Marked { 42 })
    println(atomics())
}
