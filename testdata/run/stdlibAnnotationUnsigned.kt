// Annotation classes with unsigned, unsigned-array and vararg parameters,
// and with type parameters: their instances, A(x), and the annotations,
// as the JVM's reflection reads them.

@file:OptIn(ExperimentalUnsignedTypes::class)

import kotlin.reflect.KClass

const val ONE: UInt = 1u

annotation class Unsigned(
    val b: UByte,
    val s: UShort,
    val i: UInt,
    val l: ULong,
    val max: UInt = UInt.MAX_VALUE,
    val ints: UIntArray = [1u, 2u],
    val longs: ULongArray = ulongArrayOf(3uL, *ulongArrayOf(4uL)),
    val flags: BooleanArray = [true, false],
)

annotation class Varargs(vararg val us: UInt)

annotation class Strings(val v: Array<String> = arrayOf("a", *arrayOf("b")))

annotation class Generic<T : Any>(val k: KClass<T>, val n: Int = 0)

@Unsigned(1u, 2u, 3u, 4uL)
class A

@Unsigned(255u, 65535u, ONE, ULong.MAX_VALUE, ints = uintArrayOf(5u, *uintArrayOf(6u)))
class B

@Varargs(1u, *uintArrayOf(2u, 3u))
class C

@Strings
@Generic<String>(String::class, 2)
class D

fun main() {
    val a = Unsigned(1u, 2u, 3u, 4uL)
    println(a.i)
    println(a.max)
    println(a.ints.toList())
    println(a.longs.toList())
    println(a.flags.toList())
    println(a == Unsigned(1u, 2u, 3u, 4uL))
    println(a == Unsigned(1u, 2u, 3u, 5uL))
    println(a.hashCode() == Unsigned(1u, 2u, 3u, 4uL).hashCode())
    println(a == A::class.java.getAnnotation(Unsigned::class.java))

    val b = B::class.java.getAnnotation(Unsigned::class.java)
    println(b.b)
    println(b.s)
    println(b.i)
    println(b.l)
    println(b.ints.toList())

    println(Varargs().us.size)
    println(Varargs(7u, 8u).us.toList())
    println(C::class.java.getAnnotation(Varargs::class.java).us.toList())

    println(D::class.java.getAnnotation(Strings::class.java).v.toList())
    val g = D::class.java.getAnnotation(Generic::class.java)
    println(g.k.simpleName + " " + g.n)
    val h = Generic(Int::class)
    println(h.k.simpleName + " " + h.n)
    println(h == Generic<Int>(Int::class, 0))
    println(Generic(String::class, 2) == g)
}
