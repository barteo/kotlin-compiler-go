// Local declarations that use the type parameters of the code around
// them, and type arguments written _, which the call infers.

interface Id<T> {
    fun id(x: T) = x
}

open class L<LL>(val ll: LL)

fun <T> wrap(value: T): String {
    class Local(val item: T)
    return "wrap " + Local(value).item
}

fun <T> viaLocalFun(x: T): T {
    fun g(z: T): T {
        class C : Id<T>
        return C().id(z)
    }
    return g(x)
}

fun <T, S> pair(t: T, s: S): Any = with(s) {
    data class Acc(var sum: T, var num: Int)
    Acc(t, 1)
}

fun <T1, T2, R> call2(a: T1, b: T2, f: (T1, T2) -> R): R = f(a, b)

fun <FT> ctorRef(ft: FT): L<FT> {
    class Local<LT>(lt: LT, val other: FT) : L<LT>(lt)
    return call2(ft, ft, ::Local)
}

fun <T> property(x: T): String {
    class Local(val item: T)
    val read: (Local) -> T = Local::item
    return "property " + read(Local(x))
}

class Outer<T>(v: T) {
    val prop: Any?

    init {
        class Inner(val v: T) {
            override fun toString() = "inner $v"
        }
        prop = Inner(v)
    }
}

interface Grouping<K> {
    fun keyOf(c: Char): K
}

fun <K> groupingBy(f: (Char) -> K): Grouping<K> = object : Grouping<K> {
    override fun keyOf(c: Char): K = f(c)
}

fun <K, T> both(x: (K) -> T): Pair<K, T> = Pair(1 as K, x(1 as K))

inline fun <reified A, B> named(b: B) = A::class.simpleName + " " + b

class Box<T, U>(val t: T, val u: U) {
    override fun toString() = "Box($t, $u)"
}

typealias Same<T> = Box<T, T>
typealias WithString<T> = Box<T, String>

class X {
    companion object {
        class Y {
            override fun toString() = "companion's Y"
        }
    }

    val y: Y = Y()
}

fun main() {
    println(wrap("OK"))
    println(viaLocalFun("local function"))
    println(pair(2, Any()))
    println(ctorRef("ctor").ll)
    println(property(3))
    println(Outer(4).prop)
    println(groupingBy { it.uppercaseChar() }.keyOf('a'))

    println(both<Int, _> { it.toFloat() })
    println(named<String, _>(5))
    println(Box<Int, _>(6, "u"))
    println(Same<_>(7, 8))
    println(WithString<_>(9, "s"))
    println(X().y)
}
