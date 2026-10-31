suspend fun f() {} // ERROR suspend functions need the Kotlin standard library

fun g(h: suspend () -> Unit) {} // ERROR suspend functions need the Kotlin standard library
