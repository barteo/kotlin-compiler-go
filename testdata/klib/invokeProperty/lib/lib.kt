// A library's receiver whose property has an invoke operator, as
// kotlinfixture's Generator.fixture, called as fixture() in a lambda.
package lib

class Fixture(val seed: Int) {
    inline operator fun <reified T> invoke(range: Iterable<T> = emptyList(), noinline cfg: StringBuilder.() -> Unit = {}): T {
        val sb = StringBuilder()
        sb.cfg()
        return when (T::class) {
            String::class -> "s$seed$sb" as T
            Int::class -> seed as T
            else -> range.first()
        }
    }
}

interface Generator<T> {
    val fixture: Fixture
}

class Gen<T>(seed: Int) : Generator<T> {
    override val fixture = Fixture(seed)
}

class Builder {
    val made = mutableListOf<Any?>()

    inline fun <reified T> factory(noinline f: Generator<T>.() -> T) {
        made += Gen<T>(made.size).f()
    }
}
