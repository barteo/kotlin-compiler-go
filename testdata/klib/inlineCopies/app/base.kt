// kc's public inline functions, which copy the library's: kotlinc's
// inliner copies the classes their bodies create into a caller in
// another module (use/), so they are copies of kc's own, which kotlinc
// can read where the library isn't on the class path.
package base

import lib.*

open class BaseTest {
    val fixture = Fixture()

    inline fun <reified F> make(): String = fixture<F>()

    inline fun <reified F> makeWith(noinline c: Config.() -> Unit): String = fixture<F>(c)
}

inline fun twice(x: Int): String = describe(x) { "twice ${it * 2}" }.toString()
