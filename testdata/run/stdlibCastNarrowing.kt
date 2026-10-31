// A cast x as T that a statement always evaluates, inside a call, an
// operand or an initializer, smart-casts x after it, as a test's
// assertThat((action as Feedback).title) followed by action.pills.
sealed class Action {
    class Feedback(val title: String, val pills: List<String>) : Action()

    object None : Action()
}

class Card(val action: Action?)

fun check(x: Any?) = x

fun main() {
    val card: Card? = Card(Action.Feedback("t", listOf("a")))
    val a = card?.action
    check((a as Action.Feedback).title)
    println(a.pills)
    val b: Any = "str"
    val n = (b as String).length + 1
    println(b.uppercase() + n)
    val c: Any = 3
    println("${c as Int}")
    println(c + 1)
    var d: Any? = null
    d = (card!!.action as Action.Feedback).title
    println(d)
    val e: Any? = "x"
    if (e != null) println((e as String).length)
    println(e)
    val f: Any = listOf(1, 2)
    println((f as List<*>).size)
    println(f.first())
    // A cast up to a supertype leaves the variable's own type.
    val m = mutableMapOf("a" to 1)
    println((m as Map<String, Int>).size)
    m["b"] = 2
    println(m)
}
