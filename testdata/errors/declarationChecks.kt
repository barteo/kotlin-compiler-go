interface I1
interface I2
open class Base
class Final

fun <T> repeated() where T : I1, T : I1 = Unit // ERROR repeated upper bound
fun <T> conflicting() where T : Final, T : String = Unit // ERROR have empty intersection

interface G<X>
interface GA : G<Int>
interface GB : G<String>
fun <T> inconsistent() where T : GA, T : GB = Unit // ERROR has inconsistent bounds

enum class E<T> { A } // ERROR enum class cannot have type parameters

class Finite<T : Finite<out T>> // ERROR violates the Finite Bound Restriction

interface Expansive<T> : G<Expansive<Expansive<T>>> // ERROR violates the Non-Expansive Inheritance Restriction

class Out<out T> : G<T> // ERROR occurs in

class Num<T : Number>
typealias BadRhs = Num<String> // ERROR type argument is not within its bounds
typealias NumOf<T> = Num<T>
val n: NumOf<String>? = null // ERROR in the expansion of typealias NumOf

typealias N = Nothing
fun nothingFun(): N = throw Exception() // ERROR 'Nothing' return type needs to be specified explicitly

open class Vis {
    protected var p = 1
        public set // ERROR setter visibility must be the same or less permissive
}

interface J1
interface J2
class Holder {
    val anon = object : J1, J2 {} // ERROR right-hand side has anonymous type
}

class D {
    suspend operator fun equals(other: D): Boolean = true // ERROR must not be suspend
}

fun <T> ok() where T : I1, T : I2 = Unit
class Fine<T : Comparable<T>>
