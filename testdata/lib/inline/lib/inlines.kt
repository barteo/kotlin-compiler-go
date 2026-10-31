package inl

// Inline functions that a program compiled against the library copies
// from its bytecode: kotlinc's inliner reads the reified markers and the
// invocations of lambda parameters.

inline fun <reified T> isA(x: Any?): Boolean = x is T

inline fun <reified T> castOrNull(x: Any?): T? = x as? T

inline fun <reified T> cast(x: Any?): T = x as T

inline fun <reified T : Any> javaName(): String = T::class.java.name

inline fun <reified T : Any> simpleName(): String? = T::class.simpleName

inline fun <reified T> pair(a: T, b: T): Array<T> = arrayOf(a, b)

inline fun <reified T : Enum<T>> names(): List<String> = enumValues<T>().map { it.name }

inline fun <reified T : Enum<T>> parse(name: String): T = enumValueOf<T>(name)

inline fun <reified T> filterType(xs: List<Any?>): List<T> = xs.filterIsInstance<T>()

inline fun twice(action: () -> Unit) {
    action()
    action()
}

inline fun <T> measure(block: () -> T): Pair<T, String> {
    val result = block()
    return Pair(result, "measured")
}

inline fun String.mapChars(transform: (Char) -> Char): String {
    val sb = StringBuilder()
    for (c in this) sb.append(transform(c))
    return sb.toString()
}

inline fun sum(a: () -> Int, noinline b: () -> Int): Int = a() + run(b)

inline fun <T, R> T.letIt(f: (T) -> R): R = f(this)

class Holder(val name: String) {
    inline fun <R> use(f: Holder.() -> R): R = f()
}

enum class Level { LOW, HIGH }

// Inline accessors, which a program compiled by kotlinc copies.
class Counter(var n: Int) {
    val doubled: Int inline get() = n * 2
    inline fun triple(): Int = n * 3
    var half: Int
        inline get() = n / 2
        inline set(v) {
            n = v * 2
        }
}

val String.shout: String
    inline get() = uppercase() + "!"
