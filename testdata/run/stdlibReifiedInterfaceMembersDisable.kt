// kotlinc: -jvm-default=disable
// As stdlibReifiedInterfaceMembers, with the interface's bodies in its
// DefaultImpls class: private inline functions of interfaces with reified
// type parameters, copied into the interface's other members. (Without
// the extension property, whose private accessors kc doesn't support
// there.)

import kotlin.reflect.typeOf

interface Store {
    val items: List<Any?>

    private inline fun <reified T> count(): Int = items.count { it is T }

    private inline fun <reified T> Any?.tagged(prefix: String): String =
        prefix + (if (this is T) "+" else "-") + T::class.simpleName

    private inline fun <reified T : Any> firstOrDefault(default: () -> T): T {
        for (x in items) if (x is T) return x
        return default()
    }

    private inline fun each(f: (Any?) -> Unit) {
        for (x in items) f(x)
    }

    fun summary(): String = "strings=${count<String>()} ints=${count<Int>()} nulls=${count<Nothing?>()}"

    fun tags(): String = items.joinToString { it.tagged<CharSequence>("cs") }

    fun firstString(): String = firstOrDefault { "none" }

    fun firstLong(): Long = firstOrDefault { -1L }

    // Without kotlin-reflect, whose KType prints otherwise.
    fun type(): String = typeOf<Map<String, Store?>>().let { "${it.arguments.size} ${it.arguments[1].type?.isMarkedNullable}" }

    fun firstNull(): Int {
        var i = 0
        each {
            if (it == null) return i
            i++
        }
        return -1
    }

    fun inLambda(): List<Int> = listOf(1, 2).map { count<String>() + it }

    fun inObject(): String = object {
        override fun toString() = tags()
    }.toString()
}

class Box(override val items: List<Any?>) : Store

fun main() {
    val b = Box(listOf("x", 1, null, StringBuilder("sb"), 2, "y"))
    println(b.summary())
    println(b.tags())
    println(b.firstString())
    println(Box(listOf(1)).firstString())
    println(b.firstLong())
    println(b.type())
    println(b.firstNull())
    println(Box(listOf(1)).firstNull())
    println(b.inLambda())
    println(b.inObject())
    val anon = object : Store {
        override val items = listOf<Any?>(3L)
    }
    println(anon.summary() + " " + anon.firstLong())
}
