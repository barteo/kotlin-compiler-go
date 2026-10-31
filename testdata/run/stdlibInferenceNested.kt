import kotlin.coroutines.*
import kotlin.jvm.internal.Ref.ObjectRef

// A star projection written to: Continuation<*>.resumeWith takes a
// Result<Nothing>, and so does a Cont<in T>'s resume read through Waiter<*>.
fun Continuation<*>.fail(e: Throwable) {
    resumeWith(Result.failure(e))
}

fun failOn(c: Continuation<*>, e: Throwable) {
    c.resumeWith(Result.failure(e))
}

class Res<out T>(val v: Any?) {
    override fun toString() = "Res($v)"

    companion object {
        fun <E> closed(c: String?): Res<E> = Res(c)
        fun <E> ok(e: E): Res<E> = Res(e)
    }
}

class Cont<in T>(val name: String) {
    fun resume(v: T) = println("resume $name $v")
}

class Waiter<E>(val cont: Cont<Res<E>>)

fun resumeClosed(w: Any) {
    when (w) {
        is Waiter<*> -> w.cont.resume(Res.closed("w"))
    }
}

// A branch's generic call takes the other branches' type with an expected Any.
fun pick(b: Boolean, x: Any?): Any? = if (b) Res.closed("c") else Res.ok(x)

fun pickWhen(b: Boolean): Any? = when {
    b -> emptyList()
    else -> listOf("x")
}

// A generic call passed with a lambda: its variable waits for the outer one.
fun <T> nulls(): () -> Array<T>? = { null }

@Suppress("UNCHECKED_CAST")
fun <R, T> combine(xs: Array<out T>, f: () -> Array<T?>?, g: (Array<T>) -> R): R = g(f() as Array<T>? ?: xs as Array<T>)

fun <T> box(): List<T> = emptyList()
@Suppress("UNCHECKED_CAST")
fun <T> sizes(xs: Array<out T>, f: List<T>, g: (Array<T>) -> Unit): Int {
    g(xs as Array<T>)
    return xs.size + f.size
}

// A lambda returned from let's lambda takes the outer parameter's types.
class Impl<in T> {
    fun <R> resumeImpl(update: R, mode: Int, onCancellation: ((cause: Throwable, value: R, ctx: String) -> Unit)? = null) {
        onCancellation?.invoke(RuntimeException("cancelled $mode"), update, "ctx")
    }

    fun resume(value: T, onCancellation: ((cause: Throwable) -> Unit)?) =
        resumeImpl(value, 1, onCancellation?.let { { cause, _, _ -> onCancellation(cause) } })
}

// is C<*> keeps the arguments the subject's type gives.
interface Src<out T> {
    fun get(): T
}

interface CancellableSrc<out T> : Src<T>
class CancellableImpl<T>(val s: Src<T>) : CancellableSrc<T> {
    override fun get() = s.get()
}

class DistinctSrc<T>(val s: Src<T>, val key: (T) -> Any?) : Src<T> {
    override fun get() = s.get()
}

class Plain<T>(val v: T) : Src<T> {
    override fun get() = v
}

fun <T> Src<T>.cancellable(): Src<T> = when (this) {
    is CancellableSrc<*> -> this
    else -> CancellableImpl(this)
}

fun <T> Src<T>.distinct(key: (T) -> Any?): Src<T> = when {
    this is DistinctSrc<*> && this.key === key -> this
    else -> DistinctSrc(this, key)
}

// Not those outside the class's bounds: Poly's T : Any isn't Strategy's T.
interface Strategy<T> {
    fun name(): String
}

abstract class Poly<T : Any> : Strategy<T> {
    override fun name() = "poly"
}

class PolyImpl<T : Any> : Poly<T>()
fun <T : Any> Poly<T>.find(name: String?): Strategy<T> = this

fun <T> decode(d: Strategy<T>): String {
    if (d !is Poly<*>) return d.name()
    return d.find("x").name()
}

// A typealias's constructor with type arguments, as an argument.
class Holder<T>(var value: T)
fun <T> hold(v: T) = Holder(v)

class QueueCore<E : Any>(val capacity: Int) {
    fun add(e: E) = "add $e to $capacity"

    companion object {
        const val INITIAL_CAPACITY = 8
    }
}

private typealias Core<E> = QueueCore<E>

class Queue<E : Any> {
    private val cur = hold(Core<E>(Core.INITIAL_CAPACITY))
    fun add(e: E) = cur.value.add(e)
}

// T? and T join to T?.
class Timeout<U, in T : U>(val time: Long, val cont: Cont<U>)

fun <U, T : U> setup(co: Timeout<U, T>, block: () -> T): Any? = "${co.time} ${co.cont.name} ${block()}"

@Suppress("UNCHECKED_CAST")
fun <T> withTimeout(time: Long, block: () -> T): T? {
    val tc = Timeout(time, Cont<T?>("t"))
    return setup(tc, block) as T?
}

// A Java generic class's field has its class's type argument.
class Task(val name: String)

fun steal(): Task? {
    val stolen: ObjectRef<Task?> = ObjectRef()
    stolen.element = Task("stolen")
    val result = stolen.element
    stolen.element = null
    return result
}

fun main() {
    val c = Continuation<Int>(EmptyCoroutineContext) { println(it.exceptionOrNull()?.message) }
    c.fail(RuntimeException("x"))
    failOn(c, RuntimeException("y"))
    resumeClosed(Waiter<Int>(Cont("w")))
    println(pick(true, 1))
    println(pick(false, 2))
    println(pickWhen(false))
    println(combine(arrayOf("a", "b"), nulls()) { it.joinToString() })
    println(sizes(arrayOf("a"), box()) { println(it.size) })
    Impl<String>().resume("v") { println(it.message) }
    Impl<String>().resume("v", null)
    val p = Plain("a")
    val cs = p.cancellable()
    println(cs === cs.cancellable())
    val k: (String) -> Any? = { it }
    val d = p.distinct(k)
    println(d === d.distinct(k))
    println(decode(PolyImpl<String>()))
    println(Queue<String>().add("e"))
    println(withTimeout(5) { "done" })
    println(steal()?.name)
}
