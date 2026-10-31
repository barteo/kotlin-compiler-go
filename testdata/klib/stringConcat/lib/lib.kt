// kotlinc: -jvm-target 17
package lib

// kotlinc compiles string templates for JVM 9 and later as invokedynamic
// calls of StringConcatFactory, which kc copies into its callers.

inline fun <reified T> assertAs(x: Any?, block: (T) -> Unit) {
    if (x !is T) throw AssertionError("Expected ${T::class.java.simpleName} but was $x")
    block(x)
}

inline fun describe(i: Int, l: Long, d: Double, c: Char, b: Boolean, s: String?, f: () -> String): String =
    "i=$i l=$l d=$d c=$c b=$b s=$s f=${f()}"

// A \u0001 in a template's text is the recipe's argument mark, so kotlinc
// passes the text as a constant argument.
inline fun marked(x: Int): String = "\u0001[$x]\u0002"

inline fun twice(x: Any, f: (String) -> String): String = f("$x$x") + "|" + f("<$x>")

interface Printer {
    fun print(x: Any?): String
}

inline fun <reified T> printer(): Printer = object : Printer {
    override fun print(x: Any?): String = "${T::class.java.simpleName}: $x"
}

@PublishedApi
internal fun hidden(x: String) = "hidden($x)"

inline fun published(x: Int): String = hidden("#$x")
