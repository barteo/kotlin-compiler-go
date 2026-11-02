// Inline functions that kotlinc accepts, but for the last one.

// An inline parameter passed on to an inline suspend parameter, by a
// suspend conversion, is inlined, not used as a value.
suspend inline fun suspends(f: suspend () -> String) = f()
suspend inline fun converts(f: () -> String) = suspends(f)

// Stdlib inline functions inline their lambdas, so non-local returns and
// uses of inline parameters in them are allowed.
inline fun firstOrNull(xs: List<Int>, p: (Int) -> Boolean): Int? {
    xs.forEach { if (p(it)) return it }
    return xs.firstOrNull(p)
}

inline fun <reified T> isA(x: Any) = x is T

inline val Int.twice: Int
    get() = this * 2

suspend inline fun leaks(f: () -> String): Any = f // ERROR illegal usage of inline-parameter 'f' in 'leaks'
