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
