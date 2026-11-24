class Stub<T>(val f: () -> T) {
    infix fun returns(v: T) {}
}

fun <T> stub(f: () -> T) = Stub(f)

fun use(c: JCfg) {
    stub { c.items() } returns emptyList()
    stub { c.items() } returns listOf("a")
    stub { c.all() } returns listOf("a")
    val r: List<String> = c.items()
    val m: MutableList<String> = c.items() // ERROR type mismatch
    c.items().add("b") // ERROR unresolved reference
    c.names().add("b")
    println(r + m)
}
