// Values read as a type parameter's erasure, as kotlinc casts them: not
// at all where the value is unused, after !!, and to Number to unbox a
// number; as? T checks T's bound. Nothing? from null alone yields to an
// expected type. Value classes cross erased signatures boxed: the
// arguments of an inherited $default, an inherited bridge's result,
// bridges whose value class wraps Any, Result parameters of overrides of
// generic functions, which keep their names, and the results of suspend
// overrides.

import kotlin.coroutines.*

class Key<T>(val s: String)

class Store {
    private val map = mutableMapOf<String, Any?>()
    fun <T> put(key: Key<T>, value: T) {
        map[key.s] = value
    }
    @Suppress("UNCHECKED_CAST")
    fun <T> take(key: Key<T>): T? = map.remove(key.s) as T?
}

@Suppress("UNCHECKED_CAST")
fun <T> long(): T = 1L as T

@Suppress("UNCHECKED_CAST")
fun <T> bar(t: T, r: Any?): T = r as T
fun elvis(): String? = bar(null, "elvis") ?: bar(null, "fail")

interface Shape<T>
class Square : Shape<Square>

@Suppress("UNCHECKED_CAST")
fun <T : Shape<T>> shape(a: Any): T? = a as? T

inline fun <reified T> copy(c: Collection<T>): Array<T> = c.toTypedArray()

@JvmInline
value class IC(val s: String)

interface WithDefault<T> {
    fun foo(x: T, s: String = "default"): String
}

class DefaultImpl : WithDefault<IC> {
    override fun foo(x: IC, s: String): String = x.s + " " + s
}

abstract class Holder<T> {
    var t: T? = null
    fun get(): T = t!!
}

interface GetsIC {
    fun get(): IC
}

class Inherits : Holder<IC>(), GetsIC

@JvmInline
value class AnyIC(val value: Any)

interface Face<T, R> {
    fun call(x: T): R
}

fun <T, R> apply(x: T, f: Face<T, R>): R = f.call(x)

class ResultFace : Face<Result<Int>, String> {
    override fun call(x: Result<Int>): String = "result " + x.getOrThrow()
}

open class ResultBase<T> : Face<Result<T>, String> {
    override fun call(x: Result<T>): String = "base"
}

class ResultSub : ResultBase<String>() {
    override fun call(x: Result<String>): String = "sub " + x.getOrThrow()
}

interface Producer {
    suspend fun produce(): Any
}

class ICProducer : Producer {
    override suspend fun produce(): IC = IC("produced")
}

fun run(block: suspend () -> Unit) {
    block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun main() {
    val store = Store()
    store.put(Key<Boolean>("k"), true)
    store.take(Key<String>("k"))
    println("unused")

    val x: Int = long<Int?>()!!
    println(x)

    println(elvis())

    println(shape<Square>(Any()) == null)
    println(shape<Square>(Square()) != null)

    val a: Array<String> = copy(listOf("a", "b"))
    println(a.joinToString())

    println(DefaultImpl().foo(IC("ic")))

    val h = Inherits()
    h.t = IC("inherited")
    println((h as GetsIC).get().s)

    println(apply(AnyIC(42), object : Face<AnyIC, Any> {
        override fun call(x: AnyIC): Any = x.value
    }))

    println(apply(Result.success(1), ResultFace()))
    println(ResultFace().call(Result.success(2)))
    println(apply(Result.success("ok"), ResultSub()))

    run {
        val p: Producer = ICProducer()
        println((p.produce() as IC).s)
        println(ICProducer().produce().s)
    }
}
