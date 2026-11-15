open class Inv<T, R>(x: T, r: R)
typealias Alias<X> = Inv<X, Inv<X, X>>
class InvImpl : Alias<String>("", Inv("", ""))

class Box<T : Any>
@Suppress("UPPER_BOUND_VIOLATED")
fun <T> suppressed(b: Box<T>?) = b
fun <T> violated(b: Box<T>?) = b // ERROR type argument is not within its bounds

class Constrain<A, B : A, C : B>
fun <T, U> starred(): Constrain<U, *, in T>? = null
