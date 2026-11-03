// Generic calls inferred from what their result is used as: the type of
// a cast, the other side of an elvis, a Unit a lambda returns, the
// other branches of a when and a return without a value.

fun <T> materialize(): T = "str" as T
fun <T> nullable(): T? = null
class Box<T>(val v: T) {
    fun <R> get(): R = v as R
}

sealed class Res<I, V> {
    class Val<I, V>(val value: V, val rest: I) : Res<I, V>()
    class Err<I, V>(val rest: I) : Res<I, V>()
}

fun <I, X> describe(r: Res<I, X>): Res<I, String> {
    val out = when (r) {
        is Res.Err -> Res.Err(r.rest)
        is Res.Val -> Res.Val(r.value.toString() + "!", r.rest)
    }
    return out
}

var log = ""

fun <K> record(): K {
    log += "K"
    return Unit as K
}

fun unitTail(b: Boolean) {
    42.let {
        if (b) return@let
        record()
    }
}

fun main() {
    val s = materialize() as String
    println(s.length)
    val box: Box<Any>? = Box("boxed")
    println(box?.get() as String?)
    val n: Int = nullable() ?: 7
    println(n)
    val m = nullable<Int>() ?: nullable() ?: 3
    println(m)
    val r = describe(Res.Val<Int, Int>(1, 2)) as Res.Val
    println(r.value + " " + r.rest)
    println((describe(Res.Err<Int, Int>(5)) as Res.Err).rest)
    val f: () -> Unit = { run { record() } }
    f()
    unitTail(false)
    unitTail(true)
    println(log)
}
