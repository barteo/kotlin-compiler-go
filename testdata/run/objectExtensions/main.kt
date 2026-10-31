package app

import text.Counter
import text.Formatter
import text.Formatter.Companion.shout
import text.Formatter.Companion.twice
import text.Strings.initial
import text.Strings.wrap as surround

fun main() {
    println(Formatter("> ").format("hi"))
    println("hey".shout)
    println("ab".twice())
    println(Formatter.vowels("banana"))
    println("kotlin".initial)
    println("x".surround('*'))
    val s: String? = null
    val t: String? = "z"
    println(s?.shout)
    println(t?.shout)
    println(t?.twice())
    println(Counter(5).from(1))
}
