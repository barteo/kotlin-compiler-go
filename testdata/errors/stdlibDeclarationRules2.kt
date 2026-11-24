// More rules about declarations that need the standard library: enum
// entries read before they're created, arrays of Nothing, annotations
// repeated through typealiases, named parameters of function supertypes,
// @JvmExposeBoxed names and delegates of extension properties.
@file:OptIn(ExperimentalStdlibApi::class)

import kotlin.reflect.KProperty

enum class Entries(val f: () -> Any) {
    A({ B }),
    B({ A });
    val getter get() = A
    fun member() = B
    val stored = { A }
    val inPlace = run { A } // ERROR enum entry 'A' is uninitialized here
    val inObject = object { val x = A } // ERROR enum entry 'A' is uninitialized here
    init { listOf(1).forEach { println(B) } } // ERROR enum entry 'B' is uninitialized here
    companion object { val first = A }
}
enum class Chain(val e: Chain?) {
    A(B), // ERROR enum entry 'B' is uninitialized here
    B(A),
    C(Chain.C), // ERROR enum entry 'C' is uninitialized here
    D(null) {
        init {
            println(A)
            println(D) // ERROR enum entry 'D' is uninitialized here
            println(Chain.E) // ERROR enum entry 'E' is uninitialized here
        }
    },
    E(null);
    val x = when (this) {
        A -> 1 // ERROR enum entry 'A' is uninitialized here
        else -> 2
    }
}

typealias NothingAlias = Nothing
typealias Arr<T> = Array<T>
fun nothings(
    a: Array<Nothing>, // ERROR 'Array<Nothing>' is an invalid array type
    b: Array<Nothing?>, // ERROR 'Array<Nothing?>' isn't supported on the JVM
    c: Array<out NothingAlias>?, // ERROR 'Array<Nothing>' is an invalid array type
    d: List<Array<in Nothing>>, // ERROR 'Array<Nothing>' is an invalid array type
    e: Array<List<Nothing>>,
    f: Arr<Nothing?>, // ERROR 'Array<Nothing?>' isn't supported on the JVM
) {
    arrayOf<Nothing>() // ERROR cannot use 'Nothing' as reified type parameter
    emptyList<Nothing>()
}
fun <T> generic(): Array<T> = TODO()
fun nothingCall() = generic<Nothing>() // ERROR expression cannot have a type of 'Array<Nothing>'
fun nullableCall() = generic<Nothing?>() // ERROR 'Array<Nothing?>' isn't supported on the JVM
fun stringCall() = generic<String>()
typealias ArrNothing = Arr<Nothing> // ERROR type alias expanded to malformed type
typealias ArrArrNothing = Arr<Arr<Nothing>> // ERROR type alias expanded to malformed type
typealias ArrNullable = Arr<Nothing?> // ERROR 'Array<Nothing?>' isn't supported on the JVM
typealias Unused<X, Y> = List<X>
typealias UnusedArr = Unused<Int, Array<Nothing?>>

@Target(AnnotationTarget.TYPE)
annotation class TA
@Target(AnnotationTarget.TYPE)
annotation class TB
@Target(AnnotationTarget.TYPE)
@Repeatable
annotation class TR
typealias TAInt = @TA Int
typealias TAAlias = TAInt
typealias TRInt = @TR Int
typealias TAList<T> = List<@TA T>
typealias TwiceA = @TA TAInt // ERROR this annotation is not repeatable
val twiceThrough: @TA TAAlias = 0 // ERROR this annotation is not repeatable
val inArgument: TAList<@TA Int> = listOf() // ERROR this annotation is not repeatable
val otherAnnotation: @TB TAInt = 0
val repeatable: @TR TRInt = 0
val otherArgument: TAList<@TB Int> = listOf()

abstract class FunSuper : (Int) -> Unit
abstract class NamedSuper : (x: Int) -> Unit // ERROR named parameter in function type as supertype is unsupported
abstract class NestedNamed : ((k: String) -> Unit) -> Unit

@JvmInline
value class Boxed(val s: String)
class Exposed {
    @JvmExposeBoxed("same") // ERROR the name in '@JvmExposeBoxed' cannot coincide with that of the declaration
    fun same(b: Boxed?): String = ""
    @JvmExposeBoxed("other")
    fun f(b: Boxed): String = ""
}

class Cached<T>(private val fn: (List<T>) -> T) {
    private var cache: T? = null
    operator fun getValue(thisRef: List<T>, p: KProperty<*>): T = cache ?: fn(thisRef)
}
class Stateless<T>(val n: Int) {
    operator fun getValue(thisRef: List<T>, p: KProperty<*>): T = thisRef[0]
}
class Holder<A>(val a: A) {
    operator fun getValue(thisRef: Any?, p: KProperty<*>): Int = 1
}
val <T> List<T>.cached: T by Cached { it[0] } // ERROR extension property type parameter 'T' cannot be used in delegates
val <T> List<T>.nested: Int by Holder(Cached<T> { it[0] }) // ERROR extension property type parameter 'T' cannot be used in delegates
val <T> List<T>.stateless: T by Stateless(1)

// kotlinc infers the element type from the unreachable i * 2: an Int.
fun deadLambdaTail(): String {
    while (true) {
        Array(5) { i ->
            if (i == 0) break
            return "Fail"
            i * 2
        }
    }
    return "OK"
}
