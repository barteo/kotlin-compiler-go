// x.Alias() for a typealias of an inner class needs an x of the outer
// type the alias gives, where the alias fixes its type arguments.
class Outer { inner class Inner }
typealias OI = Outer.Inner
fun test1(x: Outer) = x.OI()

class Generic<T> { inner class Inner }
typealias GI<T> = Generic<T>.Inner
typealias GIntI = Generic<Int>.Inner
fun test2(x: Generic<Int>) = x.GI()
fun <T> test3(x: Generic<T>) = x.GI()
fun <T> test5(x: Generic<T>) = x.GIntI() // ERROR unresolved reference: GIntI
fun Generic<Int>.test6() = GIntI()

open class G2<T> { inner class In }
class Sub : G2<Int>()
typealias G2I = G2<Int>.In
typealias G2N = G2<Number>.In
fun test7(s: Sub) = s.G2I()
fun test8(s: G2<Int>?) = s?.G2I()
fun test9(s: Sub) = s.G2N() // ERROR unresolved reference: G2N
fun test10(s: G2<out Int>) = s.G2I() // ERROR unresolved reference: G2I

class G3<out T> { inner class In }
typealias G3N = G3<Number>.In
fun test11(s: G3<Int>) = s.G3N()
