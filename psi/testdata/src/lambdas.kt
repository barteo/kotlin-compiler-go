fun lambdas() {
    run {
        val inner = { x: Int -> { y: Int -> x + y } }
        listOf(1).forEach { println(it) }
    }
    val f: (Int) -> Int = { it }
    with(x) { apply { also { } } }
    fun local() { { } }
    val unbalanced = { a ->
        a
    }
    foo { -> }
    foo { a, b -> }
    foo { (a, b), c -> }
    foo { a: Int -> }
    foo { @Ann x -> }
    foo { label@ x }
    foo bar@{ }
}
