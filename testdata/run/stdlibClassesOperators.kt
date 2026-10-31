import java.util.stream.Stream
import kotlin.coroutines.*

// Operators on a receiver whose type is a type parameter with a class
// bound, and extensions of a receiver with in-projected arguments.
fun <T, K, M : MutableMap<in K, in T>> List<T>.assocBy(destination: M, key: (T) -> K): M {
    for (e in this) destination[key(e)] = e
    return destination
}

fun <K, V, M : MutableMap<in K, in V>> List<Pair<K, V>>.toM(destination: M): M {
    for (e in this) destination += e
    return destination
}

fun <T, K, M : MutableMap<in K, MutableList<T>>> List<T>.group(destination: M, key: (T) -> K): M {
    for (e in this) destination.getOrPut(key(e)) { ArrayList() }.add(e)
    return destination
}

fun <T, C : MutableCollection<in T>> List<T>.into(destination: C): C {
    for (e in this) destination += e
    return destination
}

fun <T, K> List<T>.assocInto(d: MutableMap<in K, in T>, key: (T) -> K) {
    for (e in this) d[key(e)] = e
}

// An inner class of a generic superclass, in a subclass's code.
interface Clause0 {
    val obj: Any
}

interface Builder<in R> {
    fun Clause0.invoke(block: () -> R)
}

open class Sel<R>(val name: String) : Builder<R> {
    inner class Clause(val obj: Any, val block: () -> R) {
        fun run() = "$name $obj ${block()}"
    }

    override fun Clause0.invoke(block: () -> R) {
        println(Clause(obj, block).run())
    }
}

class Unbiased<R>(name: String) : Sel<R>(name) {
    private val clauses: MutableList<Clause> = arrayListOf()

    override fun Clause0.invoke(block: () -> R) {
        clauses += Clause(obj, block)
    }

    fun again(c: Clause0): Clause = this.Clause(c.obj) { error("not run") }
    fun runAll() = clauses.map { it.run() }
}

// A @JvmField property named like a Java getter.
class Worker : Thread() {
    @JvmField
    var state = 1
}

// A suspend function's JVM name with its continuation.
class Counter {
    var n = 0
    fun next(): Int = n++

    @JvmName("next")
    suspend fun next0(): Int = n++
}

// A loop over an iterator.
fun <T> Stream<T>.each(f: (T) -> Unit) {
    for (value in iterator()) f(value)
}

// The lazy message of check and requireNotNull is inlined: it can suspend,
// return and change locals.
suspend inline fun ensure(b: Boolean): Unit =
    check(b) { "missing from context $coroutineContext" }

fun firstNull(xs: List<String?>): String {
    var tries = 0
    for (x in xs) {
        requireNotNull(x) {
            tries++
            return "null after $tries"
        }
        tries++
    }
    return "none"
}

// A variable cast to an unrelated type keeps its declared type's members.
interface Frame {
    val caller: Frame?
}

interface Cont {
    val label: String
}

class Both(override val label: String, override val caller: Frame?) : Frame, Cont

private fun Frame.real(): Frame? = caller
fun use(c: Cont) = c.label

fun update(frame: Frame): String {
    val n = use(frame as Cont)
    return "$n ${frame.real()} ${frame.caller} ${frame.label}"
}

// A cast in an argument holds in the lambdas after it.
class Ch<E> {
    inline fun <R> sendImpl(element: E, waiter: Any?, onOk: () -> R, onClosed: () -> R): R =
        if (waiter == null) onClosed() else onOk()

    fun closedSend(element: E, w: Any?) = "closed $element $w"

    @Suppress("UNCHECKED_CAST")
    fun register(w: Any?, element: Any?) =
        sendImpl(element = element as E, waiter = w, onOk = { "ok" }, onClosed = { closedSend(element, w) })

    fun foo(e: E, f: () -> String) = f()

    @Suppress("UNCHECKED_CAST")
    fun register2(w: Any?, element: Any?) = foo(element as E) { closedSend(element, w) }
}

fun main() {
    println(listOf("a", "bb").assocBy(HashMap<Int, String>()) { it.length })
    println(listOf(1 to "x").toM(HashMap<Int, String>()))
    println(listOf("a", "bb", "c").group(HashMap<Int, MutableList<String>>()) { it.length })
    println(listOf(1, 2).into(ArrayList<Int>()))
    val m = HashMap<Any, Any>()
    listOf("q").assocInto(m) { it.length }
    println(m)

    val c0 = object : Clause0 {
        override val obj = "o"
    }
    with(Sel<Int>("s")) { c0.invoke { 0 } }
    val u = Unbiased<Int>("u")
    with(u) {
        c0.invoke { 1 }
        c0.invoke { 2 }
    }
    println(u.runAll())
    println(u.again(c0).obj)

    val w = Worker()
    w.state = 3
    println("${w.state} ${(w as Thread).state}")
    println(Counter().next())

    Stream.of("a", "b").each { println(it) }
    val it2: Iterator<Int> = listOf(1, 2).iterator()
    for (x in it2) println(x)

    suspend {
        ensure(true)
        try {
            ensure(false)
        } catch (e: IllegalStateException) {
            println(e.message)
        }
        println(Counter().next0())
    }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
    println(firstNull(listOf("a", null)))
    println(firstNull(listOf("a")))
    try {
        check(1 > 2) { "lazy" }
    } catch (e: IllegalStateException) {
        println(e.message)
    }

    println(update(Both("b", null)))
    println(Ch<String>().register(null, "x"))
    println(Ch<String>().register(1, "x"))
    println(Ch<String>().register2(null, "y"))
}
