// Char, Float, Byte and Short with the standard library.

fun main() {
    println("abc".first())
    println("abc".last())
    println("a,b;c".split(',', ';'))
    println("7".padStart(3, '0'))
    println('7'.digitToInt() + 1)
    println(' '.isWhitespace())
    println("  hi  ".trim())
    println("Kotlin".map { it.uppercaseChar() })
    println("hello".count { it == 'l' })
    println("abc".toCharArray().size)
    println(listOf('b', 'a').sorted())
    println("abc".reversed())
    println("x".repeat(3))
    println(listOf(1.5f, 2.5f).sum())
    println(byteArrayOf(1, 2).toList())
    println(charArrayOf('h', 'i').concatToString())
    println("q".single())
    println(listOf('a', 'b').joinToString(""))
}
