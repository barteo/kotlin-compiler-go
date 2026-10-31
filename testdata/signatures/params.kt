// Generic signatures of parameters, results, fields and accessors, with
// kotlinc's wildcards: declaration-site variance in parameters, but not
// for final types, none in results, and @JvmSuppressWildcards and
// @JvmWildcard (TestSignatures).
package p

open class Foo
class Fin
interface TypeSerializer<T>
@JvmInline value class Color(val v: Long)

fun lists(a: List<String>, b: List<Foo>, c: List<Any>, d: MutableList<String>, e: MutableList<Foo>, f: List<Fin>) {}
fun maps(a: Map<String, List<Foo>>, b: Map<String, Foo>, c: MutableMap<String, Foo>, d: Map<Foo, Fin>) {}
fun sets(a: Set<TypeSerializer<*>>, b: Set<@JvmSuppressWildcards TypeSerializer<*>>, c: Set<TypeSerializer<Foo>>) {}
fun comps(a: Comparable<Any>, b: Comparable<Foo>, c: Comparable<String>, d: Comparable<*>, e: List<*>) {}
fun funcs(a: (Int) -> Unit, b: (Foo) -> Foo, c: suspend () -> Int, d: Foo.() -> Unit, e: (String) -> Fin, f: () -> Any) {}
fun arrays(a: Array<String>, b: Array<out Foo>, c: IntArray, d: Array<Foo>, e: Array<List<Foo>>, vararg f: List<String>) {}
fun nested(a: List<List<String>>, b: List<List<Foo>>, c: List<Map<String, Foo>>, d: List<String?>, e: List<Int>, f: Map<Int, Long>) {}
fun retList(): List<Foo> = emptyList()
fun retNested(): List<List<Foo>> = emptyList()
fun retMap(): Map<String, Foo> = emptyMap()
fun retFun(): (Foo) -> Foo = { it }
fun retArr(): Array<Foo> = arrayOf()
fun retStar(): List<*> = emptyList<Any>()
fun retComp(): Comparable<Foo>? = null
fun retWild(): List<@JvmWildcard Foo> = emptyList()
fun <T> gen(t: T): List<T> = listOf(t)
fun <T : Comparable<T>> bounded(t: T): T = t
fun <T> multi(t: T) where T : CharSequence, T : Comparable<T> {}
fun <T : Foo> classBound(t: List<T>): T? = null
fun <T> unitGen(t: T) {}
fun <T> List<T>.ext(): T = first()
suspend fun susp(a: List<Foo>): List<Foo> = a
suspend fun suspPlain(): Int = 1
suspend fun <T> suspGen(t: T): T = t
fun values(c: Color, l: List<Color>, m: Map<String, Color>) {}
fun defaults(a: List<Foo> = emptyList(), b: Int = 1) {}
@JvmSuppressWildcards fun suppressed(a: List<Foo>, b: (Foo) -> Foo) {}
fun suppressParam(a: @JvmSuppressWildcards List<Foo>, b: List<Foo>) {}
fun suppressFalse(a: @JvmSuppressWildcards(false) List<Foo>) {}
fun javaTypes(a: java.util.ArrayList<Foo>, b: java.util.concurrent.atomic.AtomicReference<Foo>, c: Class<*>, d: Class<out Foo>) {}
fun outIn(a: MutableList<out Foo>, b: MutableList<in Foo>, c: Comparator<Foo>, d: Comparator<in Foo>) {}
fun nothing(a: List<Nothing>, b: Map<String, Nothing>) {}
fun pairs(a: Pair<Foo, Fin>, b: Pair<String, List<Foo>>, c: Triple<Int, Foo, Any?>) {}
fun throwsIt(a: List<Foo>): Unit = throw IllegalStateException()

val topList: List<Foo> = emptyList()
var topMap: MutableMap<String, Foo> = mutableMapOf()
val topFun: (Foo) -> Fin = { Fin() }
lateinit var topLate: Set<Foo>
private val privList: List<Foo> = emptyList()
const val CONST = 1
@JvmField val jf: List<Foo> = emptyList()

class Gen<T>(val items: List<T>, var map: Map<String, T>, private val f: (T) -> Unit) {
    lateinit var late: List<Foo>
    val fixed: List<Fin> = emptyList()
    fun get(i: Int): T = items[i]
    fun put(x: List<T>) {}
    fun <R> mapIt(f: (T) -> R): List<R> = items.map(f)
    inner class In<S>(val s: S, val t: T)
    companion object {
        val shared: List<Foo> = emptyList()
        @JvmStatic fun make(): Gen<Foo> = Gen(emptyList(), emptyMap(), {})
    }
}

class Out<out T>(val t: T) { fun get(): T = t; fun list(): List<T> = listOf(t) }
class InC<in T> { fun put(t: T) {} }

interface Repo<T> {
    fun all(): List<T>
    fun put(x: List<T>)
    fun def(x: List<Foo>): List<Foo> = x
    val prop: Map<String, T>
}

abstract class Mod {
    abstract fun bind(x: Set<TypeSerializer<*>>): Map<Class<*>, Foo>
    abstract val abs: List<Foo>
}

data class Data(val a: List<Foo>, val b: Map<String, Fin>)

object Obj { val list: List<Foo> = emptyList(); fun f(a: List<Foo>) {} }

enum class E { A; fun f(a: List<Foo>) {} }

class Ctors(val a: List<Foo>) { constructor(b: Set<Foo>, x: Int) : this(b.toList()) }

@JvmSuppressWildcards
class Suppressed(val a: List<Foo>) { fun f(a: List<Foo>): List<Foo> = a }
