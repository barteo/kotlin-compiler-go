// Calls whose candidates differ by the type of an argument that is itself a call.

import java.lang.Math.max
import java.lang.String.valueOf

class Version(val n: Int)

operator fun String.compareTo(v: Version): Int = length.compareTo(v.n)

operator fun Version.compareTo(s: String): Int = n.compareTo(s.length)

data class Entry(val version: Version, val name: String) {
    companion object {
        operator fun invoke(version: String, name: String): Entry = Entry(Version(version.length), name)
    }
}

fun name(i: Int): String = "n$i"

fun <T> id(t: T): T = t

fun entryOrNull(): Entry? =
    try {
        Entry(name(100), "b")
    } catch (_: Throwable) {
        null
    }

fun main() {
    println(name(1).compareTo(name(2)))
    println("a".compareTo(id("b")))
    println("abc".compareTo(name(10)))
    println("abc".compareTo(Version(5)))
    println("abc" < Version(5))
    println(Version(1) > "abc")
    println(name(3) < name(4))
    println(Entry(name(7), "a").version.n)
    println(Entry(Version(2), "a").version.n)
    println(entryOrNull()?.version?.n)
    println("abcd".compareTo(valueOf(max(10, 100))))
    println(Entry(Version(max(1, 2)), "e").version.n)
    val length = { i: Int -> Entry(name(i), "c").version.n }
    println(length(22))
}
