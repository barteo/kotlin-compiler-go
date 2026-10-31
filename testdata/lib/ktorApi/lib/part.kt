// A multi-file class's part: its facade has the part's constants, and
// its private functions' names end with the part's.
@file:JvmMultifileClass
@file:JvmName("Parts")

package api

const val LIMIT = 16

internal const val DOUBLE = LIMIT * 2

private fun clamp(n: Int) = minOf(n, LIMIT)

fun limited(n: Int) = clamp(n) + DOUBLE - DOUBLE
