import kotlin.reflect.KClass

// Reified type parameters with the standard library: its own reified
// functions, whose bodies are copied with the call's type arguments, and
// KClass literals of the program's.

interface Taggable {
    val tags: Map<KClass<*>, Any>

    fun <T : Any> tag(type: KClass<T>): T? = tags[type]?.let { type.java.cast(it) }
}

// As kotlinpoet's Taggable.tag().
inline fun <reified T : Any> Taggable.tag(): T? = tag(T::class)

class Spec(override val tags: Map<KClass<*>, Any>) : Taggable

inline fun <reified T> List<*>.firstOfType(): T? = firstOrNull { it is T } as T?

inline fun <reified T> List<Any?>.countOf(): Int = filterIsInstance<T>().size

fun main() {
    val xs: List<Any> = listOf("a", 1, "b", 2.0)
    println(xs.filterIsInstance<String>())
    println(xs.filterIsInstance<Int>().sum())
    println(listOf("x", "y").toTypedArray().size)
    println(xs.firstOfType<Double>())
    println(xs.countOf<String>())
    val spec = Spec(mapOf<KClass<*>, Any>(String::class to "label", Int::class to 42))
    println(spec.tag<String>())
    println(spec.tag<Int>())
    println(spec.tag<Long>())
}
