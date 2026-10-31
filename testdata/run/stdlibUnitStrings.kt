// Unit values in string templates and concatenation: kotlin.Unit, whose
// toString is "kotlin.Unit".
fun act() {
    println("act")
}

fun <T> cast(x: Any?): T = x as T

data class A(val x: Unit)

fun main() {
    val u = act()
    println("u: $u")
    println("call: ${act()}")
    println("block: ${run { act() }}")
    println("concat: " + u)
    println("concat call: " + act())
    var s = "plus:"
    s += u
    s += act()
    println(s)
    val n: Unit? = null
    println("nullable: $n ${if (n == null) Unit else n}")
    val g: Unit = cast(Unit)
    println("generic: $g")
    println("component: ${A(u).component1()} ${A(Unit)}")
}
