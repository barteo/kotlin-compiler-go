// A call without its context argument has its function's type all the
// same, for the code around it, as in K2.

context(s: String)
fun needsString(): Int = s.length

fun <R> run2(f: () -> R): R = f()

fun use() {
    val n: Int = run2 {
        needsString() // ERROR no context argument for 's: String' found
    }
}
