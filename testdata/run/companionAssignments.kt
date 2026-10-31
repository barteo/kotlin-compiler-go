// Assignments to a companion's properties by their simple names, from
// the class's initializers and methods.

class Counter {
    companion object {
        private var created = 0
        var total = 0L
        var label = ""
    }

    init {
        ++created
        total += 10
    }

    fun bump() {
        created++
        total--
        label = "bumped $created"
    }

    fun created() = created
}

class Named(n: String) {
    companion object { var last = "none" }
    init { last = n }
}

fun main() {
    val c = Counter()
    Counter()
    c.bump()
    println(c.created())
    println(Counter.total)
    println(Counter.label)
    Named("a")
    Named("b")
    println(Named.last)
}
