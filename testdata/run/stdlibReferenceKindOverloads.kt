import java.util.UUID
import kotlin.reflect.KFunction
import kotlin.reflect.KProperty

// A reference that names only functions isn't a KProperty, and one that
// names only properties isn't a KFunction: a Java class's static method,
// a class named with its package, and a companion object's member.
fun register(vararg functions: KFunction<*>) = "functions " + functions.joinToString { it.name }

fun register(vararg properties: KProperty<*>) = "properties " + properties.joinToString { it.name }

class Counter(val size: Int) {
    fun grow() = Counter(size + 1)

    companion object {
        val start = 0

        fun make() = Counter(start)
    }
}

fun main() {
    println(register(UUID::randomUUID))
    println(register(java.util.UUID::randomUUID, UUID::toString))
    println(register(Counter::grow, UUID::randomUUID))
    println(register(Counter::size))
    println(register(Counter.Companion::make))
    println(register(Counter.Companion::start))
}
