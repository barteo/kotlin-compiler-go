// The JDK's map classes take Map's compute(), computeIfPresent() and
// merge() with a function that may return null, and computeIfAbsent()
// returns the value's type.

import java.util.TreeMap
import java.util.concurrent.ConcurrentHashMap

fun fill(m: MutableMap<String, Int>) {
    m["x"] = 1
    m["w"] = 5
}

fun main() {
    val a = ConcurrentHashMap<String, Int>()
    val b = HashMap<String, Int>()
    val c = TreeMap<String, Int>()
    fill(a)
    fill(b)
    fill(c)
    a.computeIfPresent("x") { _, n -> (n - 1).takeIf { it > 0 } }
    b.computeIfPresent("x") { _, n -> (n - 1).takeIf { it > 0 } }
    c.computeIfPresent("x") { _, n -> (n - 1).takeIf { it > 0 } }
    a.compute("y") { _, n -> if (n == null) 1 else null }
    b.compute("y") { _, n -> if (n == null) 1 else null }
    c.compute("y") { _, n -> if (n == null) 1 else null }
    a.merge("w", 1) { x, y -> (x + y).takeIf { it > 9 } }
    b.merge("w", 1) { x, y -> (x + y).takeIf { it > 9 } }
    c.merge("w", 1) { x, y -> (x + y).takeIf { it > 9 } }
    val r: Int = a.computeIfAbsent("q") { 3 }
    val s: Int = b.computeIfAbsent("q") { 3 }
    val t: Int = c.computeIfAbsent("q") { 3 }
    println("${a.toSortedMap()} ${b.toSortedMap()} $c ${r + s + t}")
}
