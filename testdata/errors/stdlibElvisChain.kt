// After a ?: n ?: return, a isn't null if n always is.

fun alwaysNull(a: Throwable?, b: Unit?) {
    val c = a ?: b?.let { return it } ?: return
    throw a
}

fun nullLiteral(a: Throwable?) {
    val c = a ?: null ?: return
    throw a
}

fun maybeNotNull(a: Throwable?, b: Throwable?) {
    val c = a ?: b ?: return
    throw a // ERROR type mismatch
}

fun nullableThrow(a: Throwable?) {
    throw a // ERROR type mismatch
}

fun nullFirst(a: Throwable?, b: Unit?) {
    val c = b?.let { return it } ?: a ?: return
    throw a
}
