// A builder's lambda is analyzed on a constraint system shared with the
// calls in it, as K2's (PCLA): what the calls' own variables are known
// to be there is the builder's variables' too.

class Controller<T> {
    val items = ArrayList<T>()
    fun yield(t: T): Boolean = items.add(t)
    fun get(): T = items[0]
}

fun <S> generate(g: Controller<S>.() -> Unit): List<S> = Controller<S>().apply(g).items

fun <R> foo(w: R, x: (R) -> R): R = x(w)

fun <Q> materialize(): Q = "materialized" as Q

fun bar(x: String) {}

// S <: String, from bar(get()), is R's bound too, R <: S: the lambda's
// parameter is a String.
fun upperThroughBuilder() = generate {
    if (items.isNotEmpty()) bar(get())
    yield(foo(materialize()) { it.length.toString() })
}

// String <: S <: R: so is this one's.
fun lowerThroughBuilder() = generate {
    yield("abc")
    println(foo(get()) { it.length.toString() })
}

// The variable of a call that is an argument, K of Data(1), is fixed
// with the one it bounds, where a member needs it.
class Data<out K>(val x: K)

fun <T> next(a: T, next: (T) -> T): T = next(a)

// The right side of an elvis gives the left call's variable its type
// before the lambda is analyzed.
fun <T : Any> orNull(f: (T) -> Int): T? = null

class Owner<T> {
    var value: T? = null
    fun constrain(v: T) { value = v }
    fun provide(): T = value!!
}

fun <OT> build(lambda: (Owner<OT>) -> Unit): OT = Owner<OT>().also(lambda).provide()

fun <SOT> nestedBuild(anchor: Owner<SOT>, lambda: (Owner<SOT>) -> Unit): SOT = Owner<SOT>().also(lambda).provide()

// A builder in a builder's lambda: the inner variable is the outer one,
// and a member of its value is the String's that the outer one gave.
fun nested() = build { outer ->
    outer.constrain("outer")
    println(nestedBuild(outer) { inner ->
        inner.constrain("inner")
        println(outer.provide().length + inner.provide().length)
    })
}

fun main() {
    println(upperThroughBuilder())
    println(lowerThroughBuilder())
    println(next(Data(1)) { Data(it.x + 2) }.x)
    val length = orNull { it.length } ?: "none"
    println(length)
    println(nested())
}
