// Errors with the Kotlin standard library on the class path.

import kotlin.reflect.typeOf

class Box {
    fun get(i: Int): Int = i
}

fun <T> erased() = typeOf<T>() // ERROR cannot use 'T' as reified type parameter

fun main() {
    typeOf<String>()
    val xs = listOf(1)
    xs.add(2) // ERROR unresolved reference: add
    val ys: MutableList<Int> = xs // ERROR type mismatch: inferred type is List<Int> but MutableList<Int> was expected
    for (c in 5) { // ERROR for-loop range must have an 'iterator()' method
    }
    val p = Pair(1, 2)
    println(p[0]) // ERROR unresolved reference: get
    p.first = 3 // ERROR val cannot be reassigned
    val q: Pair<String, Int>? = null
    for (x in q) { // ERROR not nullable value required
    }
    val n: List<Int>? = null
    for (x in n) { // ERROR not nullable value required
    }
    val r: Int = 1.rangeTo(3) // ERROR type mismatch: inferred type is IntRange but Int was expected
    val chars: Int = 'a'.rangeUntil('c') // ERROR type mismatch: inferred type is CharRange but Int was expected
    val s: Int = 1.rangeTo("abc")
    println(1.rangeTo('c')) // ERROR type mismatch: inferred type is Char but String was expected
    xs[0] += 1 // ERROR unresolved reference: set
    println(Box()[0]) // ERROR 'operator' modifier is required on 'get' in 'Box'

    // Extensions, varargs and defaults.
    println(xs contains 1) // ERROR 'infix' modifier is required on 'contains'
    println("a".padStart()) // ERROR no function 'padStart' takes 0 arguments
    println(uintArrayOf("a")) // ERROR inferred type is String but UInt was expected
    println(xs.nope()) // ERROR unresolved reference: nope
    println(5.lastIndex) // ERROR unresolved reference: lastIndex
    val m = mapOf(1 to "a")
    m[2] = "b" // ERROR unresolved reference: set
    println(listOf<Int>("a")) // ERROR none of the 2 candidates for 'listOf' takes (String)
}

fun readOnly(x: Any): Boolean = x is List<*>

operator fun Int.rangeTo(s: String) = s.length
