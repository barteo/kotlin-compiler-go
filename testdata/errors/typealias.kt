typealias A = B // ERROR recursive type alias in expansion: A
typealias B = A
typealias C<T> = java.util.ArrayList<T>
typealias D<T : Any> = java.util.ArrayList<T> // ERROR bounds are not allowed on type alias parameters
class E
typealias E = Int // ERROR redeclaration: typealias E

fun f(c: C) {} // ERROR one type argument expected for typealias C<T>
fun g(x: Int): C<Int, Int>? = null // ERROR one type argument expected for typealias C<T>

interface I
open class TK<T : I, K : I>
typealias One<X> = TK<X, X>
object O : One<Any>() // ERROR type argument is not within its bounds
