// Members of Java classes and the standard library that Kotlin sees in
// its own way: Any(), nested classes of Java supertypes, a Java
// annotation's elements as properties, smart casts by the contracts of
// check and require, a Java method's type parameter inferred from a
// star-projected argument, a non-null bound with a nullable expected type,
// and a constant String's length.

import java.lang.reflect.Modifier

val ABSENT = Any()

class Entries(private val names: List<String>) : AbstractMutableMap<String, Int>() {
    override fun put(key: String, value: Int): Int? = null
    override val entries: MutableSet<MutableMap.MutableEntry<String, Int>>
        get() = names.mapIndexedTo(mutableSetOf()) { i, k -> SimpleEntry(k, i) }
}

@java.lang.Deprecated(since = "9", forRemoval = true)
class Old

fun len(x: Any): Int {
    check(x is String)
    return x.length
}

fun len2(x: Any?, y: String?): Int {
    require(x is CharSequence) { "not text" }
    requireNotNull(y)
    return x.length + y.length
}

fun hasService(c: Class<*>) = java.util.ServiceLoader.load(c).iterator().hasNext()

inline fun <reified T : CharSequence> pick(x: Any?): T? = x as? T

const val PREFIX = "Generated("
const val SIZE = PREFIX.length + 1
const val WIDE = "a😀".length

data class D(private val a: Int, val b: Int)

fun main() {
    val a = Any()
    println("${a !== ABSENT} ${a == a}")
    println(Entries(listOf("x", "y")))
    println(java.util.AbstractMap.SimpleEntry("k", 1))
    val old = Old::class.java.getAnnotation(java.lang.Deprecated::class.java)
    println("${old.since} ${old.forRemoval}")
    println("${len("four")} ${len2("ab", "c")}")
    try {
        len2(1, "c")
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(hasService(Runnable::class.java))
    var s: String? = null
    s = pick("text")
    println(s?.length)
    println("$SIZE $WIDE")
    val c1 = D::class.java.getDeclaredMethod("component1")
    println("${Modifier.isPrivate(c1.modifiers)} ${D(1, 2).component2()}")
}
