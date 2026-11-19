// Public inline members with reified type parameters, of a value class,
// a class and a member extension property, which another module inlines
// from their methods' bytecode: the -impl method of a value class's takes
// the underlying value.
package rm

@JvmInline
value class Tag(val name: String) {
    inline fun <reified T> matches(x: Any?): Boolean = x is T && name.isNotEmpty()

    inline fun <reified T : Any> label(): String = name + ":" + T::class.java.simpleName

    inline fun <reified T> filter(xs: List<Any?>): List<T> = xs.filterIsInstance<T>()

    inline fun each(f: (Char) -> Unit) {
        for (c in name) f(c)
    }
}

@JvmInline
value class Count(val n: Long) {
    inline fun <reified T> plusIf(x: Any?): Count = Count(if (x is T) n + 1 else n)
}

class Shelf(var size: Int) {
    inline val <reified T> T.kind: String
        get() = T::class.java.simpleName + size

    inline var <reified T> T.slots: Int
        get() = size + (if (this is String) 100 else 0)
        set(v) {
            size = v
        }
}

var stored = 0

// Top-level ones: their accessors' methods are synthetic, so another
// module copies their bodies, the getter's and the setter's.
inline val <reified T> T.kindTop: String
    get() = T::class.java.simpleName

inline var <reified T> T.slotTop: Int
    get() = stored + (if (this is String) 100 else 0)
    set(v) {
        stored = v
    }
