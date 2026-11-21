// The lambda of an ambiguous call is checked without the candidates'
// types, as in K2: the receiver neither candidate gives it leaves add
// unresolved.
fun <E1> g(x: MutableList<E1>.() -> Int): Int = 1
fun <E2> g(x: MutableList<E2>.() -> String): String = ""

fun main() {
    g { // ERROR ambiguous call
        undefined() // ERROR unresolved reference: undefined
        add("") // ERROR unresolved reference: add
        ""
    }
}
