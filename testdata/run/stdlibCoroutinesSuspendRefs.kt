// Callable references passed for suspend function types, as a test's
// flow.onEach(results::add): each, but String::uppercase, an inline
// function, is a FunctionReference, an object of a class of its own whose
// invoke calls the referenced function, as kotlinc's. Two to the same
// function, on the same receiver, are equal and have the same hash code.
// So are two converted to a fun interface with a suspend method, whose
// objects wrap FunctionReferences. Without an expected type, a reference to
// a suspend function is a KSuspendFunctionN, which kotlin.reflect names
// with SuspendFunctionN, and is and as check.
import kotlin.coroutines.SuspendFunction1
import kotlin.reflect.KFunction2
import kotlin.reflect.KSuspendFunction1
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.yield

fun double(x: Int) = x * 2

suspend fun compute(x: Int): Int {
    yield()
    return x + 100
}

class Box(val v: Int) {
    suspend fun plus(x: Int): Int = v + x
}

suspend fun withDefault(x: Int, y: Int = 2): Int = x + y

suspend fun sum(vararg xs: Int): Int = xs.sum()

fun same(a: Any, b: Any) = println("${a == b} ${a.hashCode() == b.hashCode()}")

suspend fun <T, R> apply(x: T, f: suspend (T) -> R): R = f(x)

fun interface Action {
    suspend fun run(x: Int): Int
}

fun interface Sink {
    suspend fun put(x: Int)
}

fun action(a: Action) = a

suspend fun record(x: Int): String {
    yield()
    println("record $x")
    return "r$x"
}

fun ks(): KSuspendFunction1<Int, Int> = ::compute

fun kinds(a: Any) = println("${a is KSuspendFunction1<*, *>} ${a is SuspendFunction1<*, *>} ${a is KFunction2<*, *, *>} ${a is Function2<*, *, *>}")

suspend fun callAll(f: SuspendFunction1<Int, Int>, k: KSuspendFunction1<Int, Int>) = f(1) + k(2) + k.name.length

fun main() = runBlocking {
    val results = mutableListOf<Int>()
    flowOf(1, 2).onEach(results::add).collect()
    println(results)
    println(flowOf(3, 4).map(::double).toList())
    println(flowOf(5).map(::compute).toList())
    println(flowOf("ab", "cde").map(String::uppercase).toList())
    println(flowOf(7).map(::Box).toList().map { it.v })
    println(apply(10, ::compute))
    val f: suspend (Int) -> Int = ::double
    println(f(21))
    val g: suspend (Int) -> Int = ::compute
    println(g(1))

    val g2: suspend (Int) -> Int = ::compute
    same(g, g2)
    same(g, f)
    val box = Box(1)
    val p1: suspend (Int) -> Int = box::plus
    val p2: suspend (Int) -> Int = box::plus
    val p3: suspend (Int) -> Int = Box(1)::plus
    same(p1, p2)
    same(p1, p3)
    println(p1(2))
    val m1: suspend (Box, Int) -> Int = Box::plus
    val m2: suspend (Box, Int) -> Int = Box::plus
    same(m1, m2)
    println(m1(box, 3))
    suspend fun local(x: Int): Int {
        yield()
        return x - 1
    }
    val l1: suspend (Int) -> Int = ::local
    val l2: suspend (Int) -> Int = ::local
    same(l1, l2)
    println(l1(4))
    val d1: suspend (Int) -> Int = ::withDefault
    val d2: suspend (Int) -> Int = ::withDefault
    same(d1, d2)
    println(d1(5))
    val s1: suspend (Int, Int) -> Int = ::sum
    val s2: suspend (Int, Int) -> Int = ::sum
    same(s1, s2)
    println(s1(6, 7))
    println(apply(8, ::compute) == apply(8, g))

    val a1 = Action(::compute)
    val a2 = action(::compute)
    same(a1, a2)
    same(a1, Action(::double))
    println(a1.run(9))
    same(Action(box::plus), Action(box::plus))
    same(Action(box::plus), Action(Box(1)::plus))
    println(Action(box::plus).run(10))
    same(action(::double), action(::double))
    println(action(::double).run(11))
    same(Action(::withDefault), Action(::withDefault))
    println(Action(::withDefault).run(12))
    same(Action(::local), Action(::local))
    println(Action(::local).run(13))
    val k1 = Sink(::record)
    same(k1, Sink(::record))
    same(k1, Sink(::compute))
    k1.put(14)

    val r1 = ::compute
    val r2 = box::plus
    val r3 = Box::plus
    println("${r1.name} ${r2.name} ${r3.name} ${ks().name}")
    same(r1, ks())
    same(r2, Box(1)::plus)
    println(r1(15) + r2(16) + r3(box, 17))
    println(callAll(r1, ks()))
    kinds(r1)
    kinds(r3)
    kinds(f)
    kinds(::double)
    val sl = suspend { x: Int -> x + 1 }
    kinds(sl)
    println(sl(18))
    println(r2 is KSuspendFunction1<Int, Any?>)
    println(sl !is KSuspendFunction1<Int, Any?>)
    println((r1 as SuspendFunction1<Int, Int>)(19))
    println((sl as suspend (Int) -> Int)(20))
}
