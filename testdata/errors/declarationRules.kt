// Rules about declarations that kotlinc's declaration checkers report:
// what an interface or class may extend, a data object's equality, the
// type of a vararg parameter, a sealed class's constructors, and a when
// over a sealed class with no subclasses.
open class Base

interface FromAny : Any() // ERROR an interface cannot inherit from a class
interface FromAnyType : Any // ERROR an interface cannot inherit from a class

abstract class MyEnum : Enum<MyEnum>("x", 0) // ERROR class 'Enum' cannot be extended directly

data object Single {
    override fun equals(other: Any?) = true // ERROR data objects cannot have custom implementations of 'equals' or 'hashCode'
    override fun hashCode() = 1 // ERROR data objects cannot have custom implementations of 'equals' or 'hashCode'
    override fun toString() = "Single"
    fun equals(other: Single) = true
}

fun nothings(vararg xs: Nothing) {} // ERROR forbidden vararg parameter type: Nothing
fun nulls(vararg xs: Nothing?) {} // ERROR forbidden vararg parameter type
fun <T : Nothing> bounded(vararg xs: T) {} // ERROR forbidden vararg parameter type
fun strings(vararg xs: String) = xs.size

sealed class Shape public constructor() { // ERROR constructors of sealed classes can only be protected or private
    internal constructor(x: Int) : this() // ERROR constructors of sealed classes can only be protected or private
    protected constructor(x: String) : this()
    private constructor(x: Long) : this()
}

sealed class Empty
sealed class One { object Only : One() }

fun empty(e: Empty): Int {
    return when (e) { // ERROR 'when' expression must be exhaustive
    }
}
fun emptyChecked(e: Empty) = when (e) { is Empty -> 1 }
fun one(o: One) = when (o) { One.Only -> 1 }

sealed fun interface SealedFun { fun f() } // ERROR 'sealed' modifier is not allowed on fun interfaces
fun interface PlainFun { fun f() }

class GenericError<T>(val t: T) : Exception() // ERROR subclass of 'Throwable' cannot have generic parameters
class Holder<T> {
    inner class InnerError : Exception() // ERROR inner class of generic class extending 'Throwable' is prohibited
    class NestedError : Exception()
}

@PublishedApi // ERROR @PublishedApi annotation is only applicable for internal declaration
fun publicApi() {}
@PublishedApi
internal fun internalApi() {}
class Api @PublishedApi constructor() { // ERROR @PublishedApi annotation is only applicable for internal declaration
    @PublishedApi private val p = 1 // ERROR @PublishedApi annotation is only applicable for internal declaration
    @PublishedApi internal val q = 1
}
class ApiParams(@PublishedApi val a: Int, @PublishedApi internal val b: Int) // ERROR @PublishedApi annotation is only applicable for internal declaration

val <T> Int.unused: Int get() = 1 // ERROR type parameter of a property must be used in its receiver type
val <T : K, K> K.boundOnly: Int get() = 1 // ERROR type parameter of a property must be used in its receiver type
interface Box<T>
val <T, K> K.throughBound: Int where K : Box<T>
    get() = 1
val <T> Box<T>.used: Int get() = 1

abstract class Shapes { abstract fun area(): Any; abstract val name: Any }
class Never : Shapes() {
    override fun area() = throw Exception()
    override val name get() = throw Exception()
    val never get() = throw Exception() // ERROR property type 'Nothing' needs to be specified explicitly
}
fun fails() = throw Exception() // ERROR return type 'Nothing' needs to be specified explicitly
val failing = null!! // ERROR property type 'Nothing' needs to be specified explicitly
fun declared(): Nothing = throw Exception()
fun locals() {
    fun local() = null!! // ERROR return type 'Nothing' needs to be specified explicitly
    val v = null!!
}
