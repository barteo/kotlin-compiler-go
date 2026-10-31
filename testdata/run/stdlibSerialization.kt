// The constructs that kotlinx.serialization's core and json needed.

@file:OptIn(ExperimentalUnsignedTypes::class)

import java.lang.reflect.*
import kotlin.native.concurrent.* // optional expectations only, as in common code
import kotlin.reflect.KClass

// Built-in classes come before star imports: java.lang.reflect.Array.
fun size(a: Array<String>): Int = a.size

// Extensions of the built-in types' companions, and an object with the
// name of a function.
interface Ser<T> { val name: String }
object CharSer : Ser<Char> { override val name = "char" }
fun Char.Companion.ser(): Ser<Char> = CharSer
object CharsSer : Ser<CharArray> { override val name = "chars" }
fun CharsSer(): Ser<CharArray> = CharsSer
object UnitSer : Ser<Unit> { override val name = "unit" }
fun Unit.ser(): Ser<Unit> = UnitSer

// Casts to function types.
fun asFun(x: Any?): ((Int) -> Int)? = x as? (Int) -> Int
fun call(x: Any): Int = (x as (Int) -> Int)(20)

// The capture of KClass<*> is an Any.
class Poly<T : Any>(val base: KClass<T>) : Ser<T> { override val name = "poly " + base.simpleName }
fun <T : Any> KClass<T>.serOrNull(): Ser<T>? = if (this == String::class) Poly(this) else null
fun <T> Ser<*>.cast(): Ser<T> = this as Ser<T>
class Cache<T>(val f: (KClass<*>) -> Ser<T>?) { fun get(k: KClass<*>): Ser<T>? = f(k) }
fun <T> createCache(factory: (KClass<*>) -> Ser<T>?): Cache<T> = Cache(factory)
val cache = createCache { it.serOrNull() ?: if (it.java.isInterface) Poly(it) else null }
fun find(c: KClass<*>): Ser<Any?>? = cache.get(c)?.cast()

// Member extensions of a generic superclass, abstract extension
// properties and their overrides.
abstract class KV<K, V, R> {
    protected abstract val R.key: K
    protected abstract val R.value: V
    open val R.label: String get() = "kv"
    abstract fun R.size(): Int
    fun describe(r: R): String = "" + r.key + "=" + r.value + " " + r.label + " " + r.size()
}
class PairKV<K, V> : KV<K, V, Pair<K, V>>() {
    override val Pair<K, V>.key: K get() = this.first
    override val Pair<K, V>.value: V get() = this.second
    override val Pair<K, V>.label: String get() = "pair " + first
    override fun Pair<K, V>.size() = 2
}

// Arrays of unsigned integers.
class UBuilder(var buffer: UShortArray) {
    var position = 0
    fun append(c: UShort) {
        if (buffer.size < position + 1) buffer = buffer.copyOf(buffer.size * 2)
        buffer[position++] = c
    }
    fun build() = buffer.copyOf(position)
}

// Annotation properties of type KClass.
@Retention(AnnotationRetention.RUNTIME)
annotation class With(val with: KClass<out Any> = Any::class)
@With(String::class)
class Annotated

// An interface's companion's constants, Nothing? results, protected
// constructor properties.
interface Decoder {
    fun decodeNull(): Nothing?
    companion object { const val DONE = -1 }
}
sealed class Sealed(protected val k: Int) : Decoder {
    override fun decodeNull(): Nothing? = null
    fun peek() = k
}
class Leaf : Sealed(7)

// A crossinline lambda invoked in another lambda, and an inherited Java
// method in an inline member's copy.
fun <T> later(f: () -> T): T = f()
class Refs : ClassValue<String>() {
    override fun computeValue(type: Class<*>): String = type.simpleName
    inline fun twice(c: Class<*>, crossinline suffix: () -> String): String = get(c) + later { suffix() }
}

// A class that implements CharSequence.
class Chars(val buffer: CharArray) : CharSequence {
    override var length: Int = buffer.size
    override fun get(index: Int): Char = buffer[index]
    override fun subSequence(startIndex: Int, endIndex: Int): CharSequence = buffer.concatToString(startIndex, minOf(endIndex, length))
}
fun firstTwo(s: CharSequence) = "" + s[0] + s[1] + s.length

// Ranges with Byte and Short bounds.
fun inByte(x: Long) = x in Byte.MIN_VALUE..Byte.MAX_VALUE

// Members that can't take a call give way to an extension of this.
interface Dec
fun Dec.cast(v: Any, name: String, path: () -> String): String = name + path() + v
abstract class TreeDec : Dec {
    fun render() = "R"
    fun cast(v: Any, name: String): String = cast(v, name) { render() }
}
class TreeDecImpl : TreeDec()

// A constant from a built-in companion's, used in a class.
private const val MAX_CODE = Char.MAX_VALUE.code
class CodePoints { fun single(c: Int) = c <= MAX_CODE }

// An inline member's default value reads its this.
class Lexer {
    var position = 7
    inline fun require(condition: Boolean, at: Int = position, message: () -> String): String = if (condition) "ok" else message() + at
}

// The lambda branch of an if gets the other branch's type.
class Consumers(val sink: (String) -> Unit) {
    var tag: String? = null
    fun consumer(): (String) -> Unit = if (tag == null) sink else { node -> sink(tag + ":" + node) }
}

// A Unit override of a generic function: its bridge returns Unit.
interface Reader<T> { fun read(): T }
object UnitReader : Reader<Unit> by object : Reader<Unit> { override fun read() {} }

// A delegated list, and a local of an inline function's copy across a
// suspension point.
class Elements(content: List<String>) : List<String> by content
inline fun braces(reader: () -> String): String {
    val sb = StringBuilder("{")
    sb.append(reader())
    sb.append("}")
    return sb.toString()
}
suspend fun DeepRecursiveScope<Int, String>.nest(n: Int): String = braces { if (n == 0) "x" else callRecursive(n - 1) }
val nested = DeepRecursiveFunction<Int, String> { nest(it) }
val unitRec = DeepRecursiveFunction<Unit, String> { "u" }

val shared = 1

fun main() {
    println(size(arrayOf("a", "b")))
    println(Char.ser().name + " " + CharsSer().name + " " + Unit.ser().name)
    println(asFun({ x: Int -> x + 1 })?.invoke(1))
    println(asFun("no"))
    println(call({ x: Int -> x * 2 }))
    println(find(String::class)?.name + " " + find(Runnable::class)?.name + " " + find(Int::class))
    println(PairKV<String, Int>().describe("b" to 2))
    val u = UBuilder(UShortArray(1))
    u.append(1u); u.append(65535u); u.append(7u)
    println(u.build().toList())
    println(Annotated::class.java.getAnnotation(With::class.java).with.simpleName)
    println(kotlin.runCatching { 3 }.getOrNull())
    val d: Decoder = Leaf()
    println("" + d.decodeNull() + " " + Decoder.DONE + " " + Leaf().peek())
    println(Refs().twice(String::class.java) { "!" })
    val cs = Chars(charArrayOf('a', 'b', 'c'))
    println(firstTwo(cs) + cs.subSequence(1, 3) + cs.indexOf('c'))
    println("" + inByte(5) + inByte(500))
    println(TreeDecImpl().cast(1, "n"))
    println("" + CodePoints().single(70000) + CodePoints().single(65))
    println(Lexer().require(false) { "bad@" })
    val c = Consumers { println(it) }
    c.consumer()("a")
    c.tag = "t"
    c.consumer()("b")
    println(UnitReader.read())
    val e = Elements(listOf("p", "q"))
    println("" + e.indexOf("q") + e.contains("p") + e.size)
    println(nested(3) + unitRec(Unit) + shared)
}
