// References to properties without accessors pass the getter their field
// would have, without the mangling of a value class type.
import kotlin.reflect.jvm.isAccessible

@JvmInline
value class S(val s: String)

class E {
    private val s = S("E")
    fun r() = ::s.apply { isAccessible = true }.call().s
}

private val top = S("T")
const val c = 3

fun main() {
    println(E().r())
    println(::top.apply { isAccessible = true }.call().s)
    println(::c.call())
}
