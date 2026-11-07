import j.Options
import j.Util
import j.options

fun pick(i: Int?, items: List<String>): String? {
    val c = if (i != null) items[i] else Util.neighbour("a", items)
    return c
}

fun pick2(i: Int?, items: List<String>): String? {
    val c = when {
        i != null -> items[i]
        else -> Util.name("x")
    }
    return c
}

fun pitch(p: Double?, o: Options) = Options.take(p ?: o.pitch)

fun branch(c: Boolean, o: Options): String {
    val x = if (c) 1.0 else o.pitch
    return Options.take(x)
}

fun unboxed(p: Double?, o: Options): Double {
    val x = p ?: o.pitch
    return x
}

fun main() {
    println(pick(null, listOf("a")))
    println(pick(0, listOf("a")))
    println(pick2(null, listOf("a")))
    println(pitch(null, options(null)))
    println(pitch(2.0, options(null)))
    println(pitch(null, options(3.0)))
    println(branch(false, options(null)))
    println(branch(true, options(null)))
    println(runCatching { unboxed(null, options(null)) }.exceptionOrNull()?.javaClass?.simpleName)
    println(unboxed(2.0, options(null)))
}
