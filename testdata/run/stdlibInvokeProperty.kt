// A call f() on an implicit receiver whose property f is of a class with
// an invoke operator: fixture() in kotlinfixture's factory { } is
// this.fixture.invoke().

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

class Text(val s: String)

fun <T> factory(seed: Int = 1, f: Generator<T>.() -> T): T = Gen<T>(seed).f()

// An invoke that is an extension, with one parameter.
class Counter(var n: Int)

operator fun Counter.invoke(by: Int): Int {
    n += by
    return n
}

class Tally {
    val count = Counter(10)
}

// A property whose invoke doesn't take the arguments gives way to a
// function of the same name further out.
fun count(s: String) = "top-level count($s)"

fun main() {
    println(factory<Text> { Text(fixture()) }.s)
    println(factory<Int>(7) { fixture<Int>() + 1 })
    println(factory<String> { fixture { append("x") } })
    println(factory<String>(3) { fixture(listOf("unused")) })
    println(factory<Double> { fixture(listOf(2.5)) })
    // Nested receivers: the inner one has no fixture, the outer has.
    println(factory<String>(5) { with(Text("t")) { s + fixture<String>() } })
    with(Tally()) {
        println(count(5))
        println(count(1))
        println(count("a"))
    }
}
