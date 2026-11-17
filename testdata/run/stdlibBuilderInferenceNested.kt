// Builder inference with nested calls and lambdas: the variables of the
// calls in a builder's lambda that its stubs relate to are inferred with
// them, as Kotlin's builder inference infers them in one system.
class Box<T> {
    val items = mutableListOf<T>()
    fun add(x: T) { items += x }
    fun first(): T = items.first()
    override fun toString() = "Box$items"
}

fun <K1> outer(builder: Box<K1>.() -> Box<K1>): Box<K1> = Box<K1>().run { builder() }

fun <K2> inner(builder: Box<K2>.() -> Unit): Box<K2> = Box<K2>().apply(builder)

interface Shape { val name: String }
interface Round : Shape
class Circle(override val name: String) : Round
class Square(override val name: String) : Shape

fun <X> check(x: X, box: Box<X>, p: (X) -> Boolean) = println("check ${p(x)}")

fun <S> generate(g: Box<S>.() -> Unit): Box<S> = Box<S>().apply(g)

fun <BK : Unit> unitBuild(transformer: (Box<BK>) -> BK): String {
    transformer(Box())
    return "unit"
}

class Pair2<A, B>(val a: A, val b: B) { override fun toString() = "($a, $b)" }

fun <A, B : A> nested(lambda: (Box<A>) -> (Box<B>) -> Unit): String {
    lambda(Box())(Box())
    return "nested"
}

fun <T> consume(x: T) = println("consumed $x")

class Out<out V>(val v: V) { override fun toString() = "Out($v)" }

fun <S> select(a: S, b: S): S = a

fun <M> none(): Out<M>? = null

abstract class Thrower<F> {
    abstract fun raise(r: F): String
}

class Named<F>(val name: String) : Thrower<F>() {
    override fun raise(r: F) = "$name raised $r"
}

fun main() {
    // A builder's lambda ending in another builder: K2 is K1, which both
    // add calls give, a String?.
    val r = outer {
        add(null)
        inner { add("a") }
    }
    println(r)

    // A lambda whose parameter type is the builder's variable: S is the
    // common supertype of both shapes.
    val round: Round = Circle("c")
    val g = generate {
        check(round, this) { x -> x is Circle }
        add(Square("s"))
    }
    println(g.items.map { it.name })

    // A result type bounded by Unit coerces the lambda's result.
    println(unitBuild { b -> b.add(Unit); "ignored" })

    // A lambda returning a lambda: both parameter types' variables are
    // inferred from the bodies, A as Any and B as String.
    println(nested { a -> { b -> if (a.items.isNotEmpty()) { consume<Any>(a.first()); consume<String>(b.first()) } } })

    // A nullable argument with variables makes the builder's type nullable.
    val n = inner { add(select(Out("o"), none())) }
    println(n)

    // An inner receiver whose member can't take the argument gives way to
    // an outer one's.
    with(Named<String>("outer")) {
        with(Named<Double>("inner")) {
            println(raise("x"))
            println(raise(1.0))
        }
    }

    // An object's private function in a builder's lambda, whose return
    // type is the builder's variable, inferred as String.
    val o = generate {
        add("first")
        object {
            fun show() = println("object ${foo()}")
            private fun foo() = first()
        }.show()
    }
    println(o)
}
