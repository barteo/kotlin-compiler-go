import kotlin.reflect.KType
import kotlin.reflect.typeOf

// typeOf of a platform type, the type of a Java value: it is not marked
// nullable, as its nullable type is, and null is of it.

inline fun <reified T> describe(x: T): String = "${typeOf<T>()} ${typeOf<T>().isMarkedNullable} ${null is T}"

inline fun <reified T> elements(x: T): KType = typeOf<List<T>>()

fun main() {
    println(describe(java.lang.String.valueOf(1)))
    println(describe("a"))
    val s: String? = System.getenv("NO_SUCH_VARIABLE")
    println(describe(s))
    println(describe(java.util.Collections.singletonList("a")))
    println(describe(listOf(java.lang.String.valueOf(1))))
    println(describe(java.util.Collections.singletonMap("a", 1)))
    println(elements(java.lang.Integer.valueOf(1)))
    println(typeOf<String>() == elements(java.lang.String.valueOf(1)).arguments[0].type)
    println(elements(java.lang.String.valueOf(1)) == elements(java.lang.String.valueOf(2)))
}
