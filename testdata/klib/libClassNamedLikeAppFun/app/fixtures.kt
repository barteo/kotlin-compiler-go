package m

fun Items(r: IntRange, flag: Boolean) = Items(own = r.toList(), other = if (flag) listOf(0) else emptyList())
