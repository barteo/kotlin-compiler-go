// Ranges as values, builder inference for the standard library's
// builders, and typeOf.

import kotlin.reflect.KClass
import kotlin.reflect.KType
import kotlin.reflect.typeOf

fun window(has: Boolean, n: Int, xs: List<String>): IntRange = if (has) 0..<n else xs.indices

fun errors(getter: Boolean, setter: Boolean): List<String> {
    val errors = buildList {
        if (!getter) {
            add("needs a getter: $getter")
        }
        if (!setter) add("needs a setter")
    }
    return errors
}

// Without kotlin-reflect, as the tests run: the KType's class, its
// nullability and its arguments.
fun describe(t: KType): String {
    val args = t.arguments.joinToString(", ") { a -> a.variance.toString() + " " + (a.type?.let { describe(it) } ?: "*") }
    return (t.classifier as KClass<*>).java.name + (if (t.isMarkedNullable) "?" else "") + "[" + args + "]"
}

inline fun <reified T> nameOf(): String = describe(typeOf<T>())

fun main() {
    val r = 1..4
    println(r)
    println(window(true, 3, listOf()) + window(false, 0, listOf("a", "b")))
    println((10 downTo 2 step 3).toList())
    println((0 until 3).sum() + (5..6).count())
    println(('a'..'d').joinToString(""))
    for (i in window(true, 2, listOf())) print(i)
    println(3 in r)
    println((r step 2).last)

    println(errors(false, false) + errors(true, true))
    println(buildSet { add(1); add(2); add(1) })
    println(buildMap { put("k", 1) })

    println(nameOf<String>())
    println(nameOf<Int?>())
    println(nameOf<List<String>>())
    println(nameOf<Map<String, List<Int>>>())
    println(nameOf<MutableList<out Number>>())
    println(nameOf<Array<String>>())
    println(nameOf<IntArray>())
    println(nameOf<Triple<Int, Long, Char>>())
    println(nameOf<Comparable<*>>())
}
