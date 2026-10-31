// An object expression's interface delegated to the enclosing code's this:
// the receiver of the lambda it is in, or the class's instance.
interface Scope {
    fun name(): String
    fun greet(who: String) = "hello $who from ${name()}"
}

interface Wide : Scope {
    fun wide(): String
}

class Base(val n: String) : Scope {
    override fun name() = n

    fun widen(): Wide = object : Wide, Scope by this {
        override fun wide() = "wide " + name()
    }
}

fun <T> withScope(s: Scope, block: Scope.() -> T): T = s.block()

fun main() {
    val w = withScope(Base("a")) {
        object : Wide, Scope by this {
            override fun wide() = "wide " + greet("w")
        }
    }
    println(w.wide())
    println(w.name())
    println(Base("b").widen().wide())
}
