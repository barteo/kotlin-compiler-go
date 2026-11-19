// Member extension properties with reified type parameters: reads copy
// the getter and writes the setter, with the dispatch receiver as this and
// the extension receiver as the receiver, as kotlinc does, through =, +=
// and ++ too.

class Counter(var n: Int) {
    var log = ""

    inline val <reified T> T.kind: String
        get() = T::class.simpleName + n

    inline var <reified T> T.count: Int
        get() = n + (if (this is String) 100 else 0)
        set(v) {
            log += "${T::class.simpleName}=$v "
            n = v
        }

    inline var <reified T> T.wide: Long
        get() = n.toLong() + (if (this is Long) 1000 else 0)
        set(v) {
            n = v.toInt()
        }

    fun inside(): String {
        1.count = 7
        "s".count += 1
        return "x".kind + " " + log
    }
}

object Registry {
    var total = 0

    inline var <reified T> T.score: Int
        get() = total + (if (this is Int) 1 else 0)
        set(v) {
            total = v
        }
}

fun counter(): Counter {
    println("counter()")
    return Counter(0)
}

fun main() {
    val c = Counter(1)
    with(c) {
        println("x".kind + " " + 2.kind + " " + listOf(1).kind)
        println("s".count)
        1.count = 5
        println(1.count)
        1.count += 2
        println(n)
        1.count++
        println(n)
        val old = 2.count--
        println("$old $n")
        println(++"a".count)
        println(5L.wide)
        5L.wide += 10L
        println(n)
        3L.wide++
        println(n)
        println(log)
    }
    println(Counter(0).inside())
    with(counter()) {
        2.count += 3
        println(n)
    }
    with(Registry) {
        1.score = 10
        1.score += 1
        "s".score++
        println(total)
    }
}
