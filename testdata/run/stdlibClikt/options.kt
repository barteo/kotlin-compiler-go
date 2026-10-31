package options

// A function that another package's has the name of: main.kt imports both.
fun Int.convert(f: (Int) -> String): String = "option " + f(this)

// A file's private property, which another file's of the same name doesn't
// conflict with.
private val conversion: Ctx.(String) -> Int = { it.toIntOrNull() ?: fail(it) }

class Ctx(val name: String) {
    fun fail(m: String): Nothing = throw IllegalStateException("$name: $m")
}

fun Ctx.int(s: String): Int = conversion(s)
