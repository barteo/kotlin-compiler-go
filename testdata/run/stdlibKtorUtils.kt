// The constructs that ktor-utils needed.

import kotlin.coroutines.*
import kotlin.reflect.*

// A member extension comes before a property of extension function type,
// which an iterator object's code can call too.
class DSet<From, To>(
    private val delegate: MutableSet<From>,
    private val convertTo: From.() -> To,
    private val convert: To.() -> From
) {
    fun Collection<To>.convert(): Collection<From> = map { it.convert() }
    fun Collection<From>.convertTo(): Collection<To> = map { it.convertTo() }
    fun addAll(elements: Collection<To>): Boolean = delegate.addAll(elements.convert())
    fun has(e: To): Boolean = delegate.contains(e.convert())
    fun first(): Iterator<To> = object : Iterator<To> {
        val it = delegate.iterator()
        override fun hasNext() = it.hasNext()
        override fun next(): To = it.next().convertTo()
    }
    override fun toString(): String = delegate.convertTo().toString()
}

// But a member of the receiver comes before a property in scope, as in
// clikt's object expressions.
class Ctx2(val k: Int)

class Opt(val each: Ctx2.(Int) -> Int)

class Holder2(val each: Ctx2.(Int) -> Int, val opt: Opt) {
    fun run(): Int = object {
        fun go() = opt.each(Ctx2(10), 1)
    }.go()
}

// x += v on what isn't a variable calls plusAssign.
class Digest {
    val parts = mutableListOf<String>()
    operator fun plusAssign(s: String) {
        parts += s
    }
}

fun Digest.build(s: String): List<String> {
    this += s
    this += s.uppercase()
    return parts
}

// A function that takes a lambda without SAM conversion comes first.
class CMap<K, V>(cap: Int) : MutableMap<K, V> by HashMap<K, V>(cap) {
    constructor() : this(8)
    fun computeIfAbsent(key: K, block: () -> V): V = getOrPut(key, block)
}

// An enum's entries, unqualified in its members and its companion's.
enum class WeekDay(val value: String) {
    MON("Mon"), TUE("Tue");
    companion object {
        fun from(ordinal: Int): WeekDay = entries[ordinal]
        fun from(value: String): WeekDay = entries.find { it.value == value } ?: error("bad $value")
    }
    fun next() = entries[(ordinal + 1) % entries.size]
}

// x !is T for a reified T leaves x a Node & T, and x === this a Node.
open class Node {
    var next: Any = this
    var prev: Any = this
    fun findHead(): Node = this
    fun removed(): String = "removed"
    fun remove(): Boolean = true
    inline fun <reified T> firstIs(): T? {
        val first = next as Node
        if (first === this) return null
        if (first !is T) return null
        if (first.remove()) return first
        return null
    }
    fun markPrev(): Node {
        val p = prev
        val r = (if (p === this) findHead() else (p as Node)).removed()
        println(r)
        return p
    }
}

class Sub : Node()

// x?.f(args) for a function value with a receiver, whose arguments are
// evaluated only if x isn't null.
class Ctx<T>(val v: T)

fun <T : Any> runIn(subject: Any, block: Ctx<T>.(T) -> String): String? {
    @Suppress("UNCHECKED_CAST")
    val re = Ctx(subject) as? Ctx<T>
    @Suppress("UNCHECKED_CAST")
    return re?.block(subject as T)
}

fun go(c: Ctx<Int>?, f: Ctx<Int>.(Int) -> Unit) {
    var n = 0
    c?.f(n++)
    println(n)
}

// === between a Continuation<T> and an object that is a Continuation<Unit>.
class Gun<T>(val conts: MutableList<Continuation<T>?>) {
    val cont: Continuation<Unit> = object : Continuation<Unit> {
        override val context: CoroutineContext
            get() {
                for (c in conts) if (c !== this && c != null) return c.context
                return EmptyCoroutineContext
            }
        override fun resumeWith(result: Result<Unit>) {}
    }
}

// A reference's name.
object Failed {
    fun failedToCapture() {}
    val prop = 1
}

class Named(val x: Int) {
    fun m(y: Int) = x + y
}

fun top() = 1

// A delegate made by a generic function is of the property's type.
class Ref<T>(private var v: T) {
    operator fun getValue(thisRef: Any?, p: KProperty<*>): T = v
    operator fun setValue(thisRef: Any?, p: KProperty<*>, value: T) {
        v = value
    }
}

fun <T> ref(initial: T): Ref<T> = Ref(initial)

class Pipe {
    private var items: List<String>? by ref(null)
    fun add(s: String) {
        items = (items ?: emptyList()) + s
    }
    override fun toString() = "$items"
}

// A try in a constructor's arguments, also in an inline function's body.
class TypeInfo(val type: KClass<*>, val kotlinType: KType?) {
    override fun toString() = "TypeInfo(${type.simpleName}, ${kotlinType != null})"
}

data class Key<T : Any>(val name: String, val info: TypeInfo)

@PublishedApi
internal inline fun <reified T> typeOfOrNull(): KType? = try {
    typeOf<T>()
} catch (_: Throwable) {
    null
}

inline fun <reified T> typeInfo(): TypeInfo = TypeInfo(T::class, typeOfOrNull<T>())

inline fun <reified T : Any> Key(name: String): Key<T> = Key(name, typeInfo<T>())

class Cookie(val name: String, val n: Int)

// A V? result, of a Kotlin or a Java function, whose V is Unit is the
// object or null: ktor's ConcurrentSet's remove(e) = map.remove(e) != null.
class UnitBox<V> {
    private var v: V? = null
    fun put(x: V) { v = x }
    fun take(): V? = v.also { v = null }
}

fun unitResults() {
    val d = java.util.concurrent.ConcurrentHashMap<String, Unit>()
    d["a"] = Unit
    println(d["a"] != null)
    println(d.remove("a") != null)
    println(d.remove("a") != null)
    val r: Unit? = d["z"]
    println(r)
    println(d.remove("z") ?: "none")
    val b = UnitBox<Unit>()
    b.put(Unit)
    println(b.take() != null)
    println(b.take() != null)
}

fun main() {
    unitResults()
    val s = DSet<Int, String>(mutableSetOf(1), { "<$this>" }, { length })
    println(s.addAll(listOf("ab", "abc")))
    println(s.has("ab"))
    println(s)
    println(s.first().next())

    println(Holder2({ it * 100 }, Opt { k + it }).run())

    println(Digest().build("ab"))
    val m = mutableMapOf("k" to mutableListOf(1))
    m["k"]!! += 2
    println(m)

    val d = CMap<String, Unit>()
    var added = false
    d.computeIfAbsent("a") { added = true }
    println(added)
    added = false
    d.computeIfAbsent("a") { added = true }
    println(added)

    println(WeekDay.from(1))
    println(WeekDay.from("Mon"))
    println(WeekDay.TUE.next())

    val n = Node()
    val sub = Sub()
    n.next = sub
    println(n.firstIs<Sub>() === sub)
    println(n.firstIs<String>())
    println(n.markPrev() === n)

    println(runIn<String>("hi") { "$v/$it" })
    go(Ctx(1)) { println(v + it) }
    go(null) { println(v + it) }

    println(Gun<String>(mutableListOf(null)).cont.context)

    println(Failed::failedToCapture.name)
    println(Named::m.name)
    println(::top.name)
    println(Failed::prop.name)
    println(::Named.name)

    val p = Pipe()
    println(p)
    p.add("a")
    p.add("b")
    println(p)

    println(Key<String>("k"))
    println(Cookie("a", runCatching { "12".toInt() }.getOrDefault(0)).n)
    println(Cookie("b", try { "x".toInt() } catch (e: NumberFormatException) { -1 }).n)
}
