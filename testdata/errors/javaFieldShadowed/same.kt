package base

fun read() = C().f // ERROR the Java field 'f' is shadowed by the Kotlin property of 'base/B'
