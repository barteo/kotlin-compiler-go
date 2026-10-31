@file:Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")

import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*
import kotlin.coroutines.jvm.internal.*
import kotlin.reflect.KFunction0
import kotlin.reflect.KFunction1
import kotlin.reflect.KFunction2

// A reference whose expected type is a KFunctionN is a KFunction with
// the function's name, and the function type it extends.
class Seg<E>(val id: Long, val prev: Seg<E>?)

fun <E> segFun(): KFunction2<Long, Seg<E>?, Seg<E>> = ::createSeg

private fun <E> createSeg(id: Long, prev: Seg<E>?) = Seg(id, prev)

fun <S> find(id: Long, start: S, create: (Long, S) -> S): S = create(id, start)

class Counter(var n: Int) {
    fun add(k: Int): Int {
        n += k
        return n
    }

    fun reset() {
        n = 0
    }
}

fun Counter.twice(k: Int) = add(k) + add(k)

// A constructor's reference is named <init>.
class Box(val v: Int) {
    inner class In(val w: Int) {
        override fun toString() = "In($v, $w)"
    }
}

fun boxRef(): KFunction1<Int, Box> = ::Box

// A class path class's internal class and members, under
// @Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE"): kotlinx.coroutines'
// SafeCollector is a ContinuationImpl.
internal class Machine(val log: MutableList<String>, done: Continuation<Any?>) : ContinuationImpl(done, EmptyCoroutineContext) {
    override val context: CoroutineContext
        get() = EmptyCoroutineContext

    override fun invokeSuspend(result: Result<Any?>): Any? {
        log += "invokeSuspend ${result.getOrNull()}"
        return COROUTINE_SUSPENDED
    }

    public override fun releaseIntercepted() {
        log += "releaseIntercepted"
        super.releaseIntercepted()
    }
}

internal fun <T> probed(c: Continuation<T>): Continuation<T> = kotlin.coroutines.jvm.internal.probeCoroutineCreated(c)

fun main() {
    val f = segFun<String>()
    println(f.name)
    println(f(3L, null).id)
    println(find(5L, null as Seg<String>?, segFun())?.id)

    val c = Counter(1)
    val add: KFunction2<Counter, Int, Int> = Counter::add
    println("${add.name} ${add(c, 2)}")
    val bound: KFunction1<Int, Int> = c::add
    println("${bound.name} ${bound(3)} ${listOf(1, 2).map(bound)}")
    val reset: KFunction0<Unit> = c::reset
    println("${reset.name} ${reset()} ${c.n}")
    val twice: KFunction2<Counter, Int, Int> = Counter::twice
    println("${twice.name} ${twice(c, 1)}")
    println("${boxRef().name} ${boxRef()(4).v}")
    val inner: KFunction2<Box, Int, Box.In> = Box::In
    println("${inner.name} ${inner(Box(1), 2)}")
    val boundInner: KFunction1<Int, Box.In> = Box(7)::In
    println("${boundInner.name} ${boundInner(3)} ${segFun<Int>() == segFun<String>()}")

    val log = mutableListOf<String>()
    val m = Machine(log, Continuation(EmptyCoroutineContext) { log += "done" })
    m.resumeWith(Result.success("x"))
    m.releaseIntercepted()
    println(log)
    println(probed(m) === m)
}
