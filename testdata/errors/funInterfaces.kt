fun interface NoMethod // ERROR fun interfaces must have exactly one abstract method

fun interface TwoMethods { // ERROR fun interfaces must have exactly one abstract method
    fun a()
    fun b()
}

fun interface WithProperty { // ERROR fun interfaces cannot have abstract properties
    val x: Int
    fun a()
}

fun interface Generic { // ERROR single abstract member cannot declare type parameters
    fun <T> a(x: T)
}

fun interface WithDefault { // ERROR single abstract member cannot declare default values
    fun a(x: Int = 1)
}

interface Plain {
    fun a()
}

fun interface Good {
    fun a(): Int
}

fun take(p: Plain) {}
fun takeGood(g: Good) = g.a()

fun use() {
    take { } // ERROR type mismatch: inferred type is () -> Unit but Plain was expected
    takeGood { 1 }
    Plain { } // ERROR cannot create an instance of an interface: Plain
}

fun constructors(g: Good, f: () -> Int) {
    Good(f)
    Good(g) // ERROR type mismatch: inferred type is Good but () -> Int was expected
    val r = ::Good
    val j = ::Runnable // ERROR Java SAM interface constructor references are prohibited: ::Runnable
    val k: (() -> Unit) -> Runnable = ::Runnable // ERROR Java SAM interface constructor references are prohibited: ::Runnable
}
