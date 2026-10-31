// Object expressions with the standard library: their class names, as
// kotlinc gives them, lambdas in their members, and generic supertypes.
interface Source<T> {
    fun next(): T
}

fun <T> repeating(values: List<T>): Source<T> = object : Source<T> {
    var i = 0
    override fun next(): T {
        i++
        return values[(i - 1) % values.size]
    }
}

class Registry {
    val names = mutableListOf<String>()
    val listener = object : Runnable {
        override fun run() {
            names.add("ran")
        }
    }

    fun watcher(prefix: String) = object : Comparator<String> {
        override fun compare(a: String, b: String) = key(a).compareTo(key(b))
        fun key(s: String) = names.size.toString() + prefix + s.length
    }
}

fun main() {
    val s = repeating(listOf("a", "b"))
    println(List(5) { s.next() })

    val reg = Registry()
    reg.listener.run()
    println(reg.names)
    println(listOf("ccc", "a", "bb").sortedWith(reg.watcher("-")))

    var count = 0
    val counter = object {
        fun inc() = run { count++ }
        val lazyText by lazy { "count is $count" }
    }
    counter.inc()
    counter.inc()
    println(counter.lazyText)
    println(listOf(1, 2, 3).map { x -> object { val sq = x * x }.sq })

    val g = object : Runnable {
        override fun run() {}
    }
    println(g.javaClass.name)
    println(s.javaClass.name)
    println(reg.listener.javaClass.name)
    println(reg.watcher("").javaClass.name)
    println(counter.javaClass.name)
}
