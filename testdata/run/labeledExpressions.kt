// Labels on expressions other than loops and lambdas: nothing can jump to
// them, so they have no effect. A label binds like a prefix operator.
class Box<T>(val value: T)

fun pick(b: Boolean) = l@ if (b) t@ "then" else e@ "else"

fun main() {
    val a = aa@ 1
    println(l1@ a + l2@ 2)
    println(n@ -a * 3)
    println(Box<Long>(x@ (1L + 2)).value)
    println(pick(true) + " " + pick(false))
    val s = (l@ "s")
    println(s2@ s.length)
    val r = t@ try {
        Integer.parseInt("x")
    } catch (e: NumberFormatException) {
        -1
    }
    println(r)
    val w = w@
        when (a) {
            1 -> "one"
            else -> "other"
        }
    println(w)
    val f = outer@ (inner@ { x: Int -> x * 2 })
    println(f(21))
}
