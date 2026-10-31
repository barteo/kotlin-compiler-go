// A library's inline functions whose bodies create its own lambdas and
// objects: a default lambda's INSTANCE, as kotlinfixture's
// Fixture.invoke(configuration = {}), and an object.
package lib

class Config {
    val items = mutableListOf<String>()
}

class Fixture {
    inline operator fun <reified T> invoke(noinline configuration: Config.() -> Unit = {}): String {
        val c = Config().apply(configuration)
        return T::class.java.simpleName + c.items
    }
}

inline fun describe(x: Int, crossinline label: (Int) -> String): Any = object {
    override fun toString() = "described " + label(x)
}
