// Two files of one multi-file class, Text: each file is a part,
// Text__StringsKt and Text__NumbersKt, and the facade Text calls them.
@file:JvmMultifileClass
@file:JvmName("Text")

package mf

fun shout(s: String): String = s.uppercase() + suffix()

private fun suffix() = "!"

@JvmOverloads
fun pad(s: String, width: Int = 6, c: Char = '.'): String = s.padEnd(width, c)

inline fun <T> twice(block: () -> T): Pair<T, T> = block() to block()
