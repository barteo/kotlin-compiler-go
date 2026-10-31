// kotlinc: -jvm-default=enable
package api

import kotlin.coroutines.AbstractCoroutineContextElement
import kotlin.coroutines.CoroutineContext

// The shapes of ktor's classes that kc wrote otherwise than kotlinc.

// Result parameters leave names unmangled, and a constructor that takes
// one public.
fun describe(r: Result<Int>): String = r.fold({ "ok $it" }, { "failed ${it.message}" })

fun Result<Int>.twice(): Int = getOrThrow() * 2

class Holder(val r: Result<Int>)

// A data class that declares a copy() still gets copy(year, month).
data class Date(val year: Int, val month: Int) {
    fun copy(): Date = Date(1970, 1)
}

// A value class's private constructor: its constructor-impl is private.
@JvmInline
value class Mode private constructor(private val bits: Int) {
    fun has(other: Mode) = bits and other.bits == other.bits

    companion object {
        val A = Mode(1)
        val B = Mode(2)
        fun of(a: Mode, b: Mode) = Mode(a.bits or b.bits)
    }
}

// Java collection superclasses: size, keys, values and entries, and
// removeAt, call theirs, and Java's methods call them.
open class Cache(private val max: Int) : LinkedHashMap<String, Int>(16, 0.75f, true) {
    override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, Int>?) = size > max
}

open class Names : ArrayList<String>()

// get overrides an operator, so it is one.
class Chars(private val a: CharArray) : CharSequence {
    override val length get() = a.size
    override fun get(index: Int) = a[index]
    override fun subSequence(startIndex: Int, endIndex: Int): CharSequence = String(a, startIndex, endIndex - startIndex)
    override fun toString() = String(a)
}

// Its superclass has the methods that call CoroutineContext.Element's
// bodies.
class Tag(val name: String) : AbstractCoroutineContextElement(Tag) {
    companion object Key : CoroutineContext.Key<Tag>
}

// The $annotations method of an interface's property is in its
// DefaultImpls.
interface Point {
    @Deprecated("use localPort")
    val port: Int
    val localPort: Int
}

// An override with another result type has no body to call in
// DefaultImpls.
interface Defaults {
    fun build(): Any = 0
}

interface Narrow : Defaults {
    override fun build(): String
}

@Deprecated("use String")
typealias Text = String

annotation class Marked

interface Source {
    val value: String
}

val source: Source = object : Source {
    @Marked
    override val value = "anon"
}
