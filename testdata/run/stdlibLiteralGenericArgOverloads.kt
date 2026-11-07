// A generic call of integer literals, passed to overloads without type
// parameters, takes its type from the candidate: or(1, 2) is a Long for
// f(Int?, Long?), and a String parameter doesn't take it.
fun <T : Any> or(a: T, b: T): T = a
fun <T> id(a: T): T = a
fun one(c: Long?) = "one $c"
fun two(c: Long?) = "two long $c"
fun two(c: String?) = "two string $c"
fun three(a: Int?, c: Long?) = "three int $a $c"
fun three(a: Long?, c: Long?) = "three long $a $c"

class Scope {
    fun <T : Any> any(): T? = null
}

fun main() {
    println(one(or(1, 2)))
    println(one(id(1)))
    println(two(or(1, 2)))
    println(three(1, or(1, 2)))
    println(three(1L, or(3, 4)))
    with(Scope()) { println(three(5, any())) }
}
