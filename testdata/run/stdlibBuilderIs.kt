open class Animal; class Dog : Animal() { fun bark() = "woof" }
fun main() {
    val l = buildList {
        add("a")
        val x = get(0)
        if (x is String) println(x.length)
        val any: Any = get(0)
        if (any is String) println(any.length)
        when (x) {
            is String -> println("s")
            is CharSequence -> println("cs")
            else -> println("else")
        }
        if (x !is CharSequence) println("never")
        println(get(0) is Comparable<*>)
    }
    println(l)
    val a = buildList<Animal> { add(Dog()) }
    val m = buildList {
        add(Dog())
        add(Animal())
        val d = get(0)
        if (d is Dog) println(d.bark())
        val e = d as? Dog
        println(e?.bark())
    }
    println(m.size + a.size)
}
