// A function-typed parameter that can't take the call's arguments gives
// way to the implicit receiver's function of its name, a lambda without
// declared parameters picks the overload whose function type takes one,
// a type parameter bounded by Unit holds the kotlin.Unit object, and
// f<T> label@{ ... } is a call with a labeled trailing lambda.

interface Sink<T> { fun emit(v: T) }
class Src<T>(val xs: List<T>)

fun <T, R> Src<T>.transform(f: Sink<R>.(T) -> Unit): List<R> {
    val out = ArrayList<R>()
    val s = object : Sink<R> { override fun emit(v: R) { out.add(v) } }
    for (x in xs) s.f(x)
    return out
}

inline fun <T, R> Src<T>.map(crossinline transform: (value: T) -> R): List<R> = transform { value ->
    return@transform emit(transform(value))
}

inline fun <T, R : Any> Src<T>.mapNotNull(crossinline transform: (value: T) -> R?): List<R> = transform { value ->
    val transformed = transform(value) ?: return@transform
    return@transform emit(transformed)
}

interface Cont<in T> {
    fun resume(value: T, onCancellation: ((cause: Throwable) -> Unit)?)
    fun <R : T> resume(value: R, onCancellation: ((cause: Throwable, value: R, ctx: String) -> Unit)?)
    fun <R : T> tryResume(value: R, idempotent: Any?, onCancellation: ((cause: Throwable, value: R, ctx: String) -> Unit)?): Any?
}

class Impl : Cont<Unit> {
    override fun resume(value: Unit, onCancellation: ((cause: Throwable) -> Unit)?) {
        println("resume(Throwable)")
        onCancellation?.invoke(Exception())
    }
    override fun <R : Unit> resume(value: R, onCancellation: ((cause: Throwable, value: R, ctx: String) -> Unit)?) {
        println("resume(Throwable, R, String)")
    }
    override fun <R : Unit> tryResume(value: R, idempotent: Any?, onCancellation: ((cause: Throwable, value: R, ctx: String) -> Unit)?): Any? {
        onCancellation?.invoke(Exception(), value, "ctx")
        return "token"
    }
}

class Wrap(val cont: Cont<Unit>) {
    fun <R : Unit> tryResume(value: R, idempotent: Any?): Any? =
        cont.tryResume(value, idempotent) { _, v, ctx -> println("cancelled $v $ctx") }
    fun <R : Unit> resume(value: R) {
        cont.resume(value) { println("unlock") }
    }
}

fun <R : Unit> same(r: R): R = r

fun <T> once(block: (Int) -> T): T = block(1)

fun labeled(): String = once<String> sc@{ n ->
    if (n > 0) return@sc "positive"
    "other"
}

fun main() {
    println(Src(listOf(1, 2, 3)).map { it * 10 })
    println(Src(listOf(1, 2, 3)).mapNotNull { if (it == 2) null else "v$it" })
    val w = Wrap(Impl())
    println(w.tryResume(Unit, null))
    w.resume(Unit)
    println(same(Unit))
    println(labeled())
}
