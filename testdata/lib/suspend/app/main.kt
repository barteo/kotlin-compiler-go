import tasks.*

suspend fun main() {
    println(sum(listOf(1, 2, 3)))
    println(sum(listOf(4), start = 100L))
    println(tail(4))
    println(Worker("w").twice(1))
    val w: Worker = Lazy("z")
    println(w.work(5))
    println(Squares().loadAll(listOf(1, 2, 3)))
    val custom = object : Store<String, Int> {
        override suspend fun load(key: String): Int = later(key.length)
    }
    println(custom.loadAll(listOf("a", "bb")))
    println(Clock.tick() + Clock.tick())
    println(lookup(Id("abc")).raw)
    println(selfTest())
    println(retry(5) { attempt -> if (attempt < 3) null else later("got $attempt") })
    val add = adder(10)
    println(add(1) + add(2))
    println(Worker("u").using { name + later(1) })
    println(timed("a") { 42 })
    println(timed("b") { later(7) * 2 })
    println(Sampler("s").sample(8))
}
