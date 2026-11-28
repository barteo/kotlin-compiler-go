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
abstract class ApiBase {
    internal abstract fun f(): Int
}
class ApiOverride : ApiBase() {
    @PublishedApi override fun f() = 1 // internal, as the function it overrides
}

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

fun interface WithDelegate {
    val functionDelegate: Any? get() = null // ERROR clashes with the function delegate
    fun invoke()
}
fun interface WithGetter {
    fun getFunctionDelegate(): Any? = null // ERROR clashes with the function delegate
    fun invoke()
}
fun interface WithOthers {
    val getFunctionDelegate: Any? get() = null
    fun functionDelegate(): Any? = null
    fun getFunctionDelegate(x: Any?): Any? = null
    fun invoke()
}

interface Root<T>
interface Child1<T> : Root<T>
interface Child2<T> : Root<T>
class StarBound<X> where X : Child2<*>, X : Child1<Base> // ERROR has inconsistent bounds
class Stars<X> where X : Child2<*>, X : Child1<*>
class OutBound<X> where X : Child2<out Base>, X : Child1<Base>

interface Delegated
class DelegatedImpl : Delegated
class NoPrimary : Delegated by DelegatedImpl() { // ERROR delegation without primary constructor
    constructor()
}
class WithPrimary() : Delegated by DelegatedImpl() {
    constructor(x: Int) : this()
}

interface Inv<T>
typealias InvStar = Inv<*>
typealias InvT<T> = Inv<T>
class FromStarAlias : InvStar // ERROR type alias with projections in its expansion
class FromProjected : InvT<*> // ERROR projections are not allowed for immediate arguments of a supertype
class FromDirect : Inv<out Base> // ERROR projections are not allowed for immediate arguments of a supertype
class FromNested : InvT<InvT<*>>
class FromPlain : InvT<Base>

inline fun withLocals(f: () -> Unit) {
    fun local() {} // ERROR local functions are not yet supported in inline functions
    val l = { fun inLambda() {} } // ERROR local functions are not yet supported in inline functions
    val anon = fun() {}
    object {
        fun member() { fun inMember() {} }
    }
}
fun notInline() { fun local() {} }

// A local variable's type parameters are an error, but their bounds are
// checked, and they aren't in scope.
fun localTypeParams() {
    val <T> a = 0 // ERROR local variables are not allowed to have type parameters
    val <T : Undefined> b = 0 // ERROR unresolved or unsupported type: Undefined
    val <T> c: T? = null // ERROR unresolved or unsupported type: T
}

// No modifier applies to an anonymous function: kotlinc keeps it.
fun anonModifiers(): Any {
    inline fun() {} // ERROR local inline functions are not yet supported
    operator fun() {} // ERROR 'operator' modifier is inapplicable on this function: illegal function name
    infix fun() {} // ERROR 'infix' modifier is inapplicable on this function
    suspend fun() {} // ERROR anonymous suspend functions are prohibited
    override fun() {} // ERROR modifier 'override' is not applicable to 'anonymous function'
    return external fun() {} // ERROR external declaration cannot have a body
}

// An integer literal out of range is an error that leaves the code
// around it checked.
fun outOfRange(): Int {
    val big = 99999999999999999999 // ERROR the value is out of range
    val s: String = 1 // ERROR type mismatch
    return big
}
