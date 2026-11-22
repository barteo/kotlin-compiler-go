// An is check of a value of a builder's variable is checked against what
// the variable is inferred as.
fun main() {
    val l = buildList {
        add("a")
        val x = get(0)
        if (x is Int) println("int") // ERROR check for instance is always 'false'
        if (x is CharSequence) println("cs")
    }
    println(l)
}
