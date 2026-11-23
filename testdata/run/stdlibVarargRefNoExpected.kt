import kotlin.reflect.KFunction1
import kotlin.reflect.KFunction2

fun count(vararg xs: Int) = xs.size
fun join(vararg xs: String) = xs.joinToString("+")
fun tail(prefix: String, vararg xs: Long, suffix: String = "!") = prefix + xs.sum() + suffix

class Box(val base: Int) {
    fun add(vararg xs: Int) = base + xs.sum()
    companion object {
        fun many(vararg xs: Char) = String(xs)
    }
}

object Obj {
    fun names(vararg xs: String) = xs.size
}

fun main() {
    val c = ::count
    println(c(intArrayOf(1, 2, 3)))
    val k: KFunction1<IntArray, Int> = c
    println(k.name)
    println(c == ::count)
    val j = ::join
    println(j(arrayOf("a", "b")))
    val t = ::tail
    println(t("s", longArrayOf(1, 2), "?"))
    val add = Box::add
    println(add(Box(10), intArrayOf(1, 2)))
    val k2: KFunction2<Box, IntArray, Int> = add
    println(k2.name)
    val bound = Box(5)::add
    println(bound(intArrayOf(4)))
    val m = Box::many
    println(m(charArrayOf('x', 'y')))
    val m2 = Box.Companion::many
    println(m2(charArrayOf('z')))
    val o = Obj::names
    println(o(arrayOf("p", "q")))
    println(listOf(intArrayOf(1), intArrayOf(1, 2)).map(::count))
    println(::count.invoke(intArrayOf(7, 8)))
}
