// Reified casts: to a non-null type argument they throw for null, with
// kotlinc's message, in the program's inline functions and in the standard
// library's objects that the program's copy.

inline fun <reified T> cast(x: Any?): T = x as T
inline fun <reified T> List<Any?>.castAll(): List<T> = map { it as T }
inline fun <reified T> List<Any?>.castSeq(): List<T> = asSequence().map { it as T }.toList()
fun main() {
    val ns = listOf<Any?>("a", null)
    println(runCatching { cast<String>(null) })
    println(runCatching { cast<String?>(null) })
    println(runCatching { cast<Map<String, Int>>(null) })
    println(runCatching { ns.castAll<String>() })
    println(runCatching { ns.castAll<String?>() })
    println(runCatching { ns.castSeq<String>() })
    println(runCatching { ns.castSeq<String?>() })
    println(runCatching { cast<Int>("x") }.exceptionOrNull()?.javaClass?.simpleName)
}
