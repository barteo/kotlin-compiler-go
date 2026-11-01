// Properties beside others of their name without context parameters,
// top-level and members, which a use refers to if their context arguments
// are found and they are visible there: read, assigned and
// compound-assigned.

class A(val n: String)

context(a: A)
var b: String
    get() = "ctx " + a.n
    set(v) {
        println("set ctx $v")
    }

private var b: String = "plain"

class W(val v: Int) {
    context(s: String)
    private val v: String
        get() = s

    context(a: A)
    var w: String
        get() = "w ctx " + a.n
        set(x) {
            println("set w ctx $x")
        }

    private var w: String = "w plain"

    fun inside(): String = v.toString() + " " + w
}

object O {
    context(a: A)
    val b: String
        get() = "O ctx " + a.n
}

fun main() {
    println(b)
    b = "x"
    b += "y"
    println(b)
    val w = W(3)
    println(w.v)
    println(w.inside())
    with(A("2")) {
        w.w = "q"
        w.w += "!"
        println(w.w)
        println(O.b)
    }
}
