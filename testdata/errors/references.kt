// Class literals and callable references.

class Box<T>(val value: T)

fun <T> classOf(x: T): String = T::class.java.name // ERROR cannot use 'T' as reified type parameter

fun f(s: String?, b: Box<Int>) {
    println(Box::class.simpleName) // ERROR class literals need the Kotlin standard library
    println(s::class.java) // ERROR expression in a class literal has a nullable type
    println(Box<Int>::class.java) // ERROR only classes are allowed on the left hand side of a class literal
    println(b::class.java.name)
    println(Box<*>::class.java.name)
    println(Missing::class.java) // ERROR unresolved reference: Missing
}

class Person(val name: String, val age: Int)

fun pick(x: Int): Int = x
fun pick(s: String): Int = 0
fun <T> id(x: T): T = x
fun many(vararg xs: Int): Int = xs.size

fun refs() {
    val a = ::pick // ERROR overload resolution ambiguity for ::pick
    val b = ::missing // ERROR unresolved reference: missing
    val c = ::id // ERROR not enough information to infer type arguments of ::id
    val d: (String) -> Int = Person::age // ERROR Person::age needs a receiver of type Person
    val e = pick // ERROR function invocation 'pick()' expected
    val f = ::many // ERROR a reference to a function with a vararg parameter needs an expected function type
    val g: (Int) -> Int = ::pick
    val h: (Int) -> String = ::pick // ERROR type mismatch
}
