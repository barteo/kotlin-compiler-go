package lib.ints

fun Int.asText() = "int $this"

val Int.size get() = "int size ${this * 2}"
