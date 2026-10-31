import rl.*

fun main() {
    println(later<String>("s")())
    println(later<Int>("s")())
    println(later<Long>(1L)())
    collect<String>(1) { println(it) }
    collect<Int>(1) { println(it) }
    println(collectAll<String>("s"))
    println(collectAll<Int>("s"))
}
