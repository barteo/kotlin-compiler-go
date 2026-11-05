// Imports of the default package's declarations by their simple names.
package app

import greet
import Counter
import Color
import limit

fun main() {
    println(greet("root"))
    val c = Counter()
    c.inc()
    c.inc()
    println(c.n)
    println(Color.GREEN)
    println(limit)
}
