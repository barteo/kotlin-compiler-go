// Intersection types: common supertypes, smart casts and inferred
// upper bounds that give A & B.
interface A
interface B { fun foo(): String }
class C : A, B { override fun foo() = "C" }
class D : A, B { override fun foo() = "D" }

fun <T> select(a: T, b: T): T = a
fun <T> id(x: T): T = x

// A declaration's type is approximated: its signature has no A & B.
fun both(c: C, d: D) = select(c, d)

abstract class P { abstract fun o(): String }
interface Q { fun k(): String }
class PQ : P(), Q {
    override fun o() = "o"
    override fun k() = "k"
}

fun cast(p: P?): String {
    if (p is Q) return id(p).o() + id(p).k() + p.k()
    return "none"
}

interface X { fun x() = "x" }
interface Z { fun z() = "z" }
interface XZ1 : X, Z
interface XZ2 : X, Z
class G<T>(private val y: T) where T : X, T : Z {
    fun check(f: (T) -> String) = f(y)
}

class In<in K>
fun <E> meet(vararg x: In<E>): E? = null

fun main() {
    val i = select(C(), D())
    println(object : B by i {}.foo())
    println(i.foo())
    val l = listOf(C(), D())
    println(l.map { it.foo() })
    println(both(C(), D()) is B)
    println(if (l.size > 1) C().foo() else D().foo())
    println(cast(PQ()))
    val g = select(G<XZ1>(object : XZ1 {}), G<XZ2>(object : XZ2 {}))
    println(g.check { it.x() + it.z() })
    val m = meet(In<Int>(), In<String>())
    println(m == null)
}
