// Secondary constructors of value classes: static constructor-impl
// methods that return the value, with defaults, varargs, early returns,
// private ones called from the companion and lambdas, and Char + String.

val log = mutableListOf<String>()

@JvmInline value class Meters(val v: Int) {
    constructor(km: Double) : this((km * 1000).toInt()) {
        log.add("km $km -> $this")
    }
    constructor(a: Int, b: Int = 10) : this(a * b)
    constructor(vararg parts: String) : this(parts.sumOf { it.length }) {
        if (parts.isEmpty()) return
        log.add("parts $v")
    }
    init {
        log.add("init $v")
    }
}

@JvmInline value class Label(val s: String) {
    private constructor(a: String, b: String) : this(a + b)
    constructor(c: Char) : this(c + "!")
    companion object {
        fun join(a: String, b: String) = Label(a, b)
    }
    fun twice() = { Label(s, s) }
}

@JvmInline value class Box<T : Any>(val items: List<T>) {
    constructor(vararg items: T) : this(items.toList())
}

fun main() {
    println(Meters(1.5))
    println(Meters(3, 4))
    println(Meters(7))
    println(Meters("a", "bc", "def"))
    println(Meters())
    println(log)
    println(Label.join("O", "K"))
    println(Label('x').twice()())
    println(Box("a", "b").items)
}
