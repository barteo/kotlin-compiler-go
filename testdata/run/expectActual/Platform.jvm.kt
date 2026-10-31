package platform

actual fun greet(name: String): String = "Hello, " + name

actual val answer: Int = 42

actual fun String.shout(): String = uppercase() + "!"

actual fun <T> firstOf(a: T, b: T): T = a

actual class Counter actual constructor(start: Int) {
    private var n = start

    actual fun next(): Int {
        n++
        return n
    }

    actual val current: Int
        get() = n
}

actual object Clock {
    actual fun now(): Long = 7L
}

actual typealias Lock = java.util.concurrent.locks.ReentrantLock
