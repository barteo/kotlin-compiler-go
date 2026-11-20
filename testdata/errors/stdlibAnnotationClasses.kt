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

// A collection literal's nested annotations infer their own type
// arguments, without the expected type, which mentions the outer type
// arguments (KT-82964); arrayOf(...), a non-generic outer annotation and
// a single nested annotation are given it.
annotation class Inner<T>
@Repeatable
annotation class Outer<T>(val arr: Array<Inner<T>>)
annotation class Outer1(val arr: Array<Inner<Int>>)
annotation class KInner<T : Any>(vararg val k: KClass<out T>)
@Repeatable
annotation class KOuter<T : Any>(val arr: Array<KInner<T>>)
annotation class KOuterOut<T : Any>(val arr: Array<KInner<out T>>)
annotation class KSingle<T : Any>(val one: KInner<T>)

@Outer<Int>([Inner()]) // ERROR argument type mismatch
fun literal1() = Unit
@Outer<Int>(arr = [Inner<String>()]) // ERROR argument type mismatch
fun literal2() = Unit
@KOuter<Any>([KInner(String::class)]) // ERROR argument type mismatch
fun literal3() = Unit
@KOuter<CharSequence>([KInner(String::class)]) // ERROR argument type mismatch
fun literal4() = Unit

@Outer<Int>([Inner<Int>()])
@Outer1([Inner()])
@KOuter<Any>([KInner<Any>(String::class)])
@KOuter<String>([KInner(String::class), KInner(String::class)])
@KOuterOut<Any>([KInner(String::class)])
@KSingle<Any>(KInner(String::class))
fun literalsFit() = Unit
