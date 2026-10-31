package arguments

fun String.convert(f: (String) -> String): String = "argument " + f(this)

private val conversion: options.Ctx.(String) -> Long = { it.toLongOrNull() ?: fail(it) }

fun options.Ctx.long(s: String): Long = conversion(s)
