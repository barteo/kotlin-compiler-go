fun <reified T> notInline(x: Any) = x is T // ERROR only type parameters of inline functions can be reified

inline fun <reified T> isA(x: Any) = x is T

fun <T> erased(x: Any) = isA<T>(x) // ERROR cannot use 'T' as reified type parameter

class Box {
    inline fun <reified T> member(x: Any) = x is T
    inline fun <reified T> Int.memberExt(x: Any) = x is T
}

@JvmInline
value class Wrapped(val n: Int) {
    inline fun <reified T> member(x: Any) = x is T
}

val <reified T> T.name: String // ERROR only type parameters of inline functions can be reified
    get() = T::class.java.name

class Host {
    val <reified T> T.name: String
        inline get() = T::class.java.name
}

open class OpenHost {
    open val <reified T> T.name: String
        inline get() = T::class.java.name // ERROR 'inline' modifier is not allowed on virtual members: only private or final members can be inlined

    open inline fun <reified T> member(x: Any) = x is T // ERROR 'inline' modifier is not allowed on virtual members: only private or final members can be inlined
}

interface Shape {
    private inline fun <reified T> member(x: Any) = x is T

    private val <reified T> T.name: String
        inline get() = T::class.java.name

    val <reified T> T.open: String
        inline get() = T::class.java.name // ERROR 'inline' modifier is not allowed on virtual members: only private or final members can be inlined

    private fun <reified T> notInline(x: Any) = x is T // ERROR only type parameters of inline functions can be reified
}
