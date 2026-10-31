import inl.*

fun main() {
    println("" + isA<String>("s") + " " + isA<String>(1) + " " + isA<Int>(1))
    println("" + castOrNull<String>(1) + " " + castOrNull<String>("x") + " " + cast<Int>(5))
    println(javaName<String>() + " " + javaName<Level>() + " " + simpleName<Holder>())
    println(pair("a", "b").joinToString() + " " + pair(1, 2).javaClass.simpleName)
    println(names<Level>().toString() + " " + parse<Level>("HIGH"))
    println(filterType<String>(listOf(1, "a", null, "b")))
    var n = 0
    twice { n++ }
    println(n)
    println(measure { 6 * 7 })
    println("abc".mapChars { it.uppercaseChar() })
    println(sum({ 1 }, { 2 }))
    println(5.letIt { it * it })
    println(Holder("h").use { name + "!" })
    val c = Counter(10)
    c.half = 7
    println("" + c.n + " " + c.doubled + " " + c.half + " " + c.triple() + " " + "hi".shout)
}
