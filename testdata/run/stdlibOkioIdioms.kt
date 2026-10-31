// Code that okio 3.18 compiles with: calls that the package's own
// functions don't fit, Long ranges, smart casts after !!, Java
// properties, builder inference from an extension's receiver, and a try
// in this(...)'s arguments.

import java.io.File
import java.io.RandomAccessFile
import java.security.MessageDigest
import java.util.GregorianCalendar
import kotlin.reflect.KClass
import kotlin.reflect.cast

// The package's minOf(Long, Int), require(Int) and String(Int) don't take
// these calls: the standard library's do.
internal fun minOf(a: Long, b: Int): Long = if (a < b) a else b.toLong()
internal fun require(x: Int): Int = x
internal fun String(x: Int): String = "s$x"

// An infix extension of Int, which Int's and takes over its own.
internal infix fun Int.and(other: Long): Long = toLong() and other

class Peek(private val buffer: StringBuilder) {
    fun length() = buffer.length
}

fun Peek.buffer(): String = "the extension"

class Buf {
    fun require(byteCount: Long): Long = byteCount

    fun check(b: Byte, n: Long): String {
        require(n >= 0L) { "n < 0" } // the standard library's
        return Peek(StringBuilder("ab")).buffer() + " " + b.toString(16) + " " + require(n)
    }
}

fun ranges(i: Int, n: Long): Long {
    val w: Long = 8 * 1024
    var s = 0L
    if (n in 0L until n + 1) s += 1
    if (n in 0..n) s += 2
    for (j in 0L until i) s += j
    for (j in 0 until n) s += j
    for (j in 10L downTo 8L step 2) s += j
    val r = i and 0xffffffffL shl 32 or (i and 0xffffffffL)
    when (n) {
        in 0..3 -> s += 100
        else -> s += 1000
    }
    return s + w + r
}

class Seg(var next: Seg?, val shared: Boolean) {
    fun copy() = Seg(null, false)
}

fun asserted(readWrite: Boolean, s: Seg?): String {
    var next = s
    if (readWrite && next!!.shared) {
        next = next.copy()
    }
    return "shared=${next?.shared}"
}

fun <T : Any> extra(type: KClass<out T>, value: Any): T? = type.cast(value)

class Options(val xs: Array<out String>)

fun options(vararg xs: String) = Options(xs.copyOf())

class Watchdog : Thread("watchdog") {
    init {
        isDaemon = true // Thread's setDaemon
    }
}

class Segmented(val parts: List<ByteArray>) {
    fun digest(): Int = MessageDigest.getInstance("MD5").run {
        // An extension of the class's this, inside a lambda with another
        // receiver.
        forEachPart { update(it) }
        digest().size
    }
}

inline fun Segmented.forEachPart(action: (ByteArray) -> Unit) {
    for (p in parts) action(p)
}

suspend fun SequenceScope<String>.words(n: Int) {
    for (i in 1..n) yield("w$i")
}

class Hashing(val name: String) {
    constructor(name: String, bad: Boolean) : this(
        try {
            if (bad) throw IllegalArgumentException("bad") else name
        } catch (e: IllegalArgumentException) {
            "caught ${e.message}"
        },
    )
}

fun main() {
    println(minOf(3L, 4L) + minOf(5, 6))
    println(String(byteArrayOf(65, 66), Charsets.UTF_8))
    println(Buf().check(31, 2))
    println(ranges(3, 2))
    println(asserted(true, Seg(null, true)))
    println(extra(String::class, "s"))
    println(options("a", "b").xs.size)
    println(Watchdog().isDaemon)
    println(GregorianCalendar().time.time > 0) // Calendar's getTime(), not its protected field
    val f = File.createTempFile("okio", "idioms")
    RandomAccessFile(f, "r").use { println(it.fd.valid()) } // getFD()
    f.delete()
    println(Segmented(listOf(byteArrayOf(1))).digest())
    println(sequence { words(3) }.toList())
    println(Hashing("x", false).name + ", " + Hashing("x", true).name)
}
