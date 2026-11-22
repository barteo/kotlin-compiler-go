// The lambda of an ambiguous call is checked without an expected type, as
// kotlinc does: its body has no receiver and no it.
fun f(x: StringBuilder.() -> Int): Int = 1
fun f(x: StringBuilder.() -> String): String = ""
fun g(x: (StringBuilder) -> Int): Int = 1
fun g(x: (StringBuilder) -> String): String = ""
fun <E1> bl(x: MutableList<E1>.() -> Int): Int = 1
fun <E2> bl(x: MutableList<E2>.() -> String): String = ""

fun test() {
    f { // ERROR ambiguous call: 2 candidates for 'f'
        append("") // ERROR unresolved reference: append
        this.length // ERROR 'this' is not defined in this context
        ""
    }
    g { // ERROR ambiguous call: 2 candidates for 'g'
        it.append("") // ERROR unresolved reference: it
        ""
    }
    g { sb -> // ERROR ambiguous call: 2 candidates for 'g'
        ""
    }
    bl { // ERROR ambiguous call: 2 candidates for 'bl'
        add("") // ERROR unresolved reference: add
        ""
    }
}
