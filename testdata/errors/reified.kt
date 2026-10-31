fun <reified T> notInline(x: Any) = x is T // ERROR only type parameters of inline functions can be reified

inline fun <reified T> isA(x: Any) = x is T

fun <T> erased(x: Any) = isA<T>(x) // ERROR cannot use 'T' as reified type parameter

class Box {
    inline fun <reified T> member(x: Any) = x is T
    inline fun <reified T> Int.memberExt(x: Any) = x is T
}

@JvmInline
value class Wrapped(val n: Int) {
    inline fun <reified T> member(x: Any) = x is T // ERROR reified type parameters of members of interfaces and value classes are not supported
}

val <reified T> T.name: String // ERROR only type parameters of inline functions can be reified
    get() = T::class.java.name

class Host {
    val <reified T> T.name: String // ERROR reified type parameters of member extension properties are not supported
        inline get() = T::class.java.name
}
