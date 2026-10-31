package platform

import kotlin.js.JsName

expect fun greet(name: String): String

expect val answer: Int

expect fun String.shout(): String

expect fun <T> firstOf(a: T, b: T): T

expect class Counter(start: Int) {
    fun next(): Int
    val current: Int
}

expect object Clock {
    fun now(): Long
}

expect class Lock() {
    fun lock()
    fun unlock()
}

fun <T> Lock.withLock(f: () -> T): T {
    lock()
    try {
        return f()
    } finally {
        unlock()
    }
}

// An annotation only JavaScript has, which the JVM drops.
@JsName("jsTwice")
fun twice(x: Int) = x * 2
