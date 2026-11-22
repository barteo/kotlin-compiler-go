class Box<T> { var v: T? = null; fun put(t: T) { v = t }; fun take(): T = v!!; fun peek(): T? = v }
fun <T> box(f: Box<T>.() -> Unit): Box<T> = Box<T>().apply(f)

fun main() {
    val l = buildList {
        add("a")
        val x = get(0)!!
        println(x.length)
        val y: Any = get(0)
        if (y is String) println(y.length)
        val self = this
        println(self.size)
        this.add("b")
        println(this[1]!!.length)
    }
    println(l)
    val b = box {
        put(3)
        val p = peek()!!
        println(p + 1)
        val q = peek()
        if (q != null) println(q + 2)
        val t = this
        println(t.take())
        println(take()!!)
    }
    println(b.take())
    val m = buildMap {
        put("k", 1)
        val e = this["k"]!!
        println(e + 1)
        val g = get("k")
        if (g != null) println(g.inc())
    }
    println(m)
}
