// Inline members of value classes, with and without reified type
// parameters: each call copies the body, with the receiver's underlying
// value as this, as kotlinc does, so lambdas return from the caller.

import kotlin.reflect.typeOf

@JvmInline
value class Name(val s: String) {
    inline fun <reified T> isA(x: Any?): Boolean = x is T && s.isNotEmpty()

    // Without kotlin-reflect, whose KType prints otherwise.
    inline fun <reified T> type(): String = typeOf<T>().let { "${it.arguments.size} ${it.isMarkedNullable}" } + "/" + s

    inline fun <reified T> cast(x: Any?): T = x as T

    inline fun <reified T> safeCast(x: Any?): T? = x as? T

    inline fun <reified T> pair(x: T): Array<T> = arrayOf(x, x)

    inline fun forEachChar(f: (Char) -> Unit) {
        for (c in s) f(c)
    }

    inline fun <reified T : Any> describe(): String = T::class.simpleName + " in " + this + " " + length()

    fun length(): Int = s.length

    // A member's own copies: this is the underlying value.
    fun selfCheck(): String = "${isA<String>(s)} ${describe<Long>()}"
}

@JvmInline
value class Id(val v: Long) {
    inline fun <reified T> plus(x: Any?): Long = v + (if (x is T) 1 else 0)

    inline val <reified T> T.tagged: String
        get() = "$v:" + (this is Number)

    fun tags(): String = 1.tagged + " " + "s".tagged
}

@JvmInline
value class Maybe(val s: String?) {
    inline fun <reified T> orKind(): String = s ?: T::class.simpleName!!
}

class Holder(val n: Name)

fun firstVowel(n: Name): Char {
    n.forEachChar { if (it in "aeiou") return it }
    return '-'
}

fun main() {
    val n = Name("kc")
    println("${n.isA<String>("x")} ${n.isA<Int>("x")} ${Name("").isA<String>("x")}")
    println(n.type<List<String>>() + " " + n.type<String?>())
    println("${n.cast<String>("ok")} ${n.safeCast<Int>("no")}")
    println(n.pair(3).size)
    println(n.describe<Map<*, *>>())
    println(n.selfCheck())
    println("${firstVowel(Name("xyz"))} ${firstVowel(Name("hello"))}")
    val nn: Name? = n
    println(nn?.isA<CharSequence>("x"))
    val none: Name? = null
    println(none?.type<Int>())
    val a: Any = n
    if (a is Name) println(a.describe<Int>())
    println(Holder(n).n.type<Unit>())
    println(listOf(Name("a"), Name("bb")).map { it.describe<String>() })
    val id = Id(41)
    println(id.plus<Int>(1) + id.plus<String>(1))
    println(id.tags())
    with(id) { println(2L.tagged) }
    println(Maybe(null).orKind<Short>() + " " + Maybe("m").orKind<Short>())
    val m: Maybe? = Maybe(null)
    println(m?.orKind<Byte>())
}
