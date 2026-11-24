package d

@Platform("actual")
actual class Paired actual constructor(n: Int) {
    actual val n: Int = n
    actual val String.width: Int get() = length
    actual fun f(a: Int): String = ""
    actual class Nested {
        actual fun g() {}
    }
    fun extra() {}
}

actual interface Port {
    actual fun read(): String
}

actual object Hub : Port {
    actual override fun read(): String = ""
}

actual typealias Text = StringBuilder

actual annotation class Note(actual val text: String)

@JvmInline
actual value class Key(actual val raw: String)

@Platform("actual")
actual fun paired(a: Int): Paired = Paired(a)

actual fun over(a: Int): Int = a
actual fun String.over(a: Int): Int = a
actual fun <U> over(a: U, b: List<U>): U = a

@Platform("actual")
actual val pairedVal: Paired = Paired(1)
actual val String.pairedVal: Int get() = 1
actual var <T> List<T>.pairedSize: Int
    get() = size
    set(value) {}
actual val lazily: Int by lazy { 1 }

class UsesToo(val p: Paired, val text: Text)
