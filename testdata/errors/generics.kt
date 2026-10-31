class Box<T>(var value: T)
class NonNull<T : Any>(val v: T)
class Two<A, B>(val a: A, val b: B)

fun <T> id(x: T): T = x
fun <T> none(): T? = null
fun plain(x: Int): Int = x

fun f1(b: Box<Int, Int>) {} // ERROR one type argument expected for class Box<T>
fun f2(b: Box) {} // ERROR one type argument expected for class Box<T>
fun f3(s: String<Int>) {} // ERROR no type arguments expected for class String
fun f4(t: Two<Int>) {} // ERROR 2 type arguments expected for class Two<A, B>
fun f5(n: NonNull<String?>) {} // ERROR type argument is not within its bounds
fun <T> f6(x: T): Any = x // ERROR inferred type is T but Any was expected
fun <T> f7(x: T) {
    val y: T = null // ERROR inferred type is Nothing? but T was expected
    x.hashCode() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed
    println(x!!.hashCode())
    val z: T = x
}
fun <T> f8(x: T<Int>) {} // ERROR type arguments are not allowed for type parameters

fun main() {
    val n = NonNull(null) // ERROR inferred type Nothing? of type parameter 'T' is not a subtype of Any
    val b: Box<Any> = Box<String>("a") // ERROR inferred type is Box<String> but Box<Any> was expected
    val c = id<Int, Int>(1) // ERROR one type argument expected for 'id<T>'
    val d = plain<Int>(1) // ERROR no type arguments expected for 'plain'
    val e = none() // ERROR cannot infer type for type parameter 'T'
    val g = Box<String>(1) // ERROR inferred type is Int but String was expected
    val h: Box<Long> = Box("s") // ERROR inferred type is Box<String> but Box<Long> was expected
    val k: Box<Int> = Box(1)
    k.value = "s" // ERROR inferred type is String but Int was expected
    println<Int>(1) // ERROR no type arguments expected for 'println'
}

class Producer<out T>(val v: T, var w: T) { // ERROR type parameter T is declared as 'out' but occurs in 'invariant' position
    fun take(x: T) {} // ERROR type parameter T is declared as 'out' but occurs in 'in' position
    fun give(): T = v
    fun boxed(): Box<T>? = null // ERROR occurs in 'invariant' position in type Box<T>?
}
class Consumer<in T> {
    fun take(x: T) {}
    fun give(): T? = null // ERROR type parameter T is declared as 'in' but occurs in 'out' position
    fun nested(p: Producer<T>) {}
}

fun variance(p: Producer<String>, b: Box<out Number>, c: Box<in Int>, s: Box<*>) {
    val a: Producer<Any> = p
    val n: Number = b.value
    b.value = 1 // ERROR inferred type is Int but Nothing was expected
    c.value = 2
    val x: Int = c.value // ERROR inferred type is Any? but Int was expected
    val y: Any? = s.value
    val list = java.util.ArrayList<String>()
    println(list.size()) // ERROR expression 'size' of type Int cannot be invoked as a function
    list.add(1) // ERROR inferred type is Int but String! was expected
    val m: java.util.HashMap<String, Int> = java.util.HashMap()
    val v: Int = m.get("a") // ERROR inferred type is Int? but Int was expected
}

fun <T : T> selfBound() {} // ERROR type parameter T has cyclic upper bounds
fun <E : E & Any> selfBoundDnn(e: E?) {} // ERROR type parameter E has cyclic upper bounds
class Cyclic<A : B, B : A> // ERROR type parameter A has cyclic upper bounds
fun <R : T & Any, T> notCyclic() {}

fun <T> twoClasses(x: T) where T : Number, T : Exception {} // ERROR only one of the upper bounds can be a class
fun <T, U> boundedByParam(x: T) where T : U, T : Comparable<T> {} // ERROR type parameter cannot have any other bounds if it's bounded by another type parameter
fun <T> outOfBounds(x: T) where T : CharSequence, T : Comparable<T> {}
fun useOutOfBounds() = outOfBounds(1) // ERROR type argument is not within its bounds: inferred type Int of type parameter 'T' is not a subtype of CharSequence
