// A class in a builder's lambda that delegates an interface whose type
// has the builder's variable.
interface Box<T> {
    fun get(): T
    val size: Int
    fun twice(): List<T> = listOf(get(), get())
}

class Holder<T>(private val v: T) : Box<T> {
    override fun get() = v
    override val size get() = 1
}

fun main() {
    val l = buildList {
        add("a")
        val h = Holder(get(0))
        class D : Box<String> by h
        val o = object : Box<String> by h {
            override fun get() = h.get() + "!"
        }
        add(D().get())
        addAll(D().twice())
        add(o.get() + o.size)
    }
    println(l)
    val m = buildList {
        add(1)
        val d = object : Box<Int> by Holder(first()) {
            val head = this@buildList.get(0)
            fun both() = listOf(head, get())
        }
        add(d.get() + d.size)
        addAll(d.both())
    }
    println(m)
}
