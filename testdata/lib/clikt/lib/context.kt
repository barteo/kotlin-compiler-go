package lib

import kotlin.properties.ReadOnlyProperty
import kotlin.reflect.KProperty

// An inline member with a reified type parameter and a default value, and
// a lambda in its body that uses it: copiers create copies of its class.
class Context(val parent: Context?, val data: MutableMap<String, Any?> = mutableMapOf()) {
    fun selfAndAncestors(): Sequence<Context> = generateSequence(this) { it.parent }
    inline fun <reified T : Any> findObject(key: String = DEFAULT_KEY): T? =
        selfAndAncestors().mapNotNull { it.data[key] as? T }.firstOrNull()

    companion object {
        const val DEFAULT_KEY = "obj"
        const val TERMINAL_KEY = "terminal"
    }
}

// SAM-converted lambdas in inline functions, as classes.
inline fun <reified T : Any> Context.requireObject(key: String = Context.DEFAULT_KEY): ReadOnlyProperty<Any?, T> =
    ReadOnlyProperty { _, _ -> findObject<T>(key)!! }

// A crossinline default value that captures the receiver, and a lambda
// that captures it and a parameter.
open class Command(val commandName: String) {
    val actions = mutableListOf<() -> Unit>()
    val registered = mutableListOf<String>()
}
fun <T : Command> T.eager(names: Collection<String>, help: String = "", action: () -> Unit): T {
    actions += action
    return this
}
inline fun <T : Command> T.versionOption(
    version: String,
    help: String = "Show the version",
    names: Set<String> = setOf("--version"),
    crossinline message: (String) -> String = { "$commandName version $it" },
): T = eager(names, help) { println(message(version)) }

// A lambda captured by a lambda, and a capturing one in a default value.
inline fun <V> Command.convert(
    noinline metavar: () -> String = { commandName.uppercase() },
    crossinline conversion: (String) -> V,
): (String) -> Pair<String, V> {
    val transform: (String) -> V = { conversion(it.trim()) }
    return { metavar() to transform(it) }
}

// An operator provideDelegate, which registers the property.
class Option(val value: String) {
    operator fun provideDelegate(thisRef: Command, property: KProperty<*>): ReadOnlyProperty<Command, String> {
        thisRef.registered += property.name
        return ReadOnlyProperty { _, _ -> value }
    }
}
fun option(value: String) = Option(value)

// A protected nested class, which subclasses use.
abstract class Formatter<P> {
    protected class Section<P>(val title: String, val content: P)
    protected abstract fun sections(): List<Section<P>>
    fun render(): String = sections().joinToString { "${it.title}=${it.content}" }
}

// Properties of function type, which implicit receivers' calls invoke.
interface Transforms<T> {
    val transformValue: (String) -> T
    val transformEach: (List<T>) -> T
}
