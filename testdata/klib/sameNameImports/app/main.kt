import lib.ints.asText
import lib.ints.size
import lib.strings.asText
import lib.strings.size

// Extensions of one name imported from two packages of a library are
// overloads.
fun main() {
    println("a".asText())
    println(1.asText())
    println("abc".size)
    println(4.size)
}
