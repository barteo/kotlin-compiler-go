// Imports of typealiases, built-in functions, and members through them.
package app

import defs.Colors.*
import defs.Outer.Name
import kotlin.arrayOfNulls
import kotlin.intArrayOf

fun main() {
    val n: Name = "name"
    println(n)
    println(GREEN)
    println(RED.ordinal)
    println(arrayOfNulls<String>(2).size)
    println(intArrayOf(1, 2, 3)[2])
}
