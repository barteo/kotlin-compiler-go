// Unit as a reified type argument, as mockk<Unit>() in a test's
// Response.success(mockk()) for a Response<Unit>: its casts and checks
// are of the class kotlin.Unit.
inline fun <reified T> cls() = T::class.java

inline fun <reified T> make(): T = T::class.java.getField("INSTANCE").get(null) as T

inline fun <reified T> isA(x: Any?) = x is T

class Response<T>(val body: T) {
    companion object {
        fun <T> success(body: T) = Response(body)
    }
}

fun main() {
    println(cls<Unit>())
    val u: Unit = make()
    println(u)
    val r: Response<Unit> = Response.success(make())
    println(r.body)
    println(listOf(isA<Unit>(Unit), isA<Unit>("x")))
}
