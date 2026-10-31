// A library's inline function whose crossinline parameter its body
// captures in a lambda, as a test helper featureFixture(configuration):
// the program's inline functions pass their own crossinline parameters on
// to it.
package lib

class Builder {
    val items = mutableListOf<String>()

    fun add(s: String) {
        items += s
    }
}

fun setup(block: Builder.() -> Unit): List<String> = Builder().apply(block).items

inline fun <reified F> common(crossinline configuration: Builder.() -> Unit = { }): String {
    val items = setup {
        add(F::class.java.simpleName)
        configuration()
    }
    return items.toString()
}

inline fun twice(crossinline action: (Int) -> String): List<String> = listOf(1, 2).map { action(it) }
