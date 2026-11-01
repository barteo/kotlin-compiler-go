// Annotation classes' parameters of unsigned types and arrays, and their
// type parameters.

import kotlin.reflect.KClass

annotation class Unsigned(val u: UInt, val us: UIntArray, val bs: BooleanArray, vararg val v: ULong)
annotation class A1(val us: Array<UInt>) // ERROR invalid type of annotation member
annotation class A2(val u: UInt?) // ERROR annotation parameters cannot be nullable
annotation class A3(val us: List<UInt>) // ERROR invalid type of annotation member
annotation class Generic<T : Any>(val k: KClass<T>, val n: Int = 0)

@Generic<String>(String::class)
@Unsigned(1u, [2u], [true], 3uL, 4uL)
class UsesThem

fun instances() = Generic(Int::class) == Generic<Int>(Int::class, 1)

// An annotation entry doesn't infer its type arguments, unlike a call.
@Generic(String::class) // ERROR one type argument expected for annotation class Generic<T>
class Inferred

@Generic<String, Int>(String::class) // ERROR one type argument expected for annotation class Generic<T>
class TooMany

@Unsigned<Int>(1u, [2u], [true]) // ERROR no type arguments expected for annotation class Unsigned
class NotGeneric

@Generic<Int>(Int::class, n = 0)
@Unsigned(1u, [2u], [true], *ulongArrayOf(3uL))
class Spread

// A vararg of unsigned values is an unsigned array; of an unsigned array
// or another value class it is prohibited.
fun unsigned(vararg us: UInt) {}
fun nullable(vararg ns: UInt?) {}
fun arrays(vararg a: UIntArray) {} // ERROR prohibited vararg parameter type 'UIntArray'

typealias FixedGeneric = Generic<Int>

@FixedGeneric(Int::class)
class Aliased
