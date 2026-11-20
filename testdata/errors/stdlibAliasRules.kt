typealias A = B // ERROR recursive type alias in expansion: A
typealias B = A // ERROR recursive type alias in expansion: B
typealias F1 = (Int) -> F2 // ERROR recursive type alias in expansion: F1
typealias F2 = (F1) -> Int // ERROR recursive type alias in expansion: F2

typealias ToParam<T> = T // ERROR typealias should expand to a class
typealias ToParam2<T> = ToParam<T>

class In<in T>
class Out<out T>
typealias In3<T> = In<out T> // ERROR projection is conflicting
fun inOut(x: In3<out Int>) = x // ERROR conflicting
typealias Out1<T> = Out<T>
fun outIn(x: Out1<in Int>) = x // ERROR conflicting
fun outOut(x: Out1<Int>) = x

class C<T>
typealias CStar = C<*>
typealias CIn = C<in Int>
typealias CT<T> = C<T>
val c1 = CStar() // ERROR type alias with projections in its expansion can't be used as a constructor
val c2 = CIn() // ERROR type alias with projections in its expansion can't be used as a constructor
val c3 = CT<CT<*>>()

class Pair2<T1, T2>(val x1: T1, val x2: T2)
typealias PR<T1, T2> = Pair2<T2, T1>
val p1 = PR<Int, String>(1, "") // ERROR type mismatch
val p2 = PR<String, Int>(1, "")
val p3: Pair2<Int, String> = PR<String, Int>(1, "")

interface I
open class TK<T : I, K : I>
typealias One<X> = TK<X, X>
typealias OneList<X> = List<TK<X, X>>
class A1<T : One<Any>> // ERROR not within its bounds
interface IO1 : OneList<Any> // ERROR not within its bounds

open class GB<T> { open fun foo() {} }
typealias GBInt = GB<Int>
typealias GBAny<T> = GB<T>
class Sub<T> : GB<T>() {
    override fun foo() {
        super<GB>.foo()
        super<GBAny>.foo()
        super<GBInt>.foo() // ERROR not an immediate supertype
    }
}

class KTest {
    class Nested
    companion object {
        val ok = 1
    }
}
typealias KT = KTest
val n1: KT.Nested? = null // ERROR unresolved
val n2 = KT.ok
val n3 = KTest.Nested()
