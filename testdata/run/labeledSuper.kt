// super@Outer and super<I>@Outer: an outer class's super call, from an
// inner class, a local class, an object expression and its delegate,
// through accessors of the outer class. Also when entries on one line
// and a trailing lambda after an index.

interface Named {
    fun name(): String = "Named"
}

open class Base {
    open fun name(): String = "Base"
    open var count: Int = 1
    open val tag: String get() = "base"
    open fun self(): Named = object : Named {
        override fun name() = "Base.self"
    }
}

open class Outer : Base(), Named {
    override fun name(): String = "Outer"
    override var count: Int = 10
    override val tag: String get() = "outer"

    inner class Inner : Base() {
        override fun name(): String = "Inner"

        fun names(): String =
            super@Outer.self().name() + " " + super<Named>@Outer.name() + " " + super<Base>@Outer.name() +
                " " + super.name() + " " + super@Inner.name()

        fun props(): String {
            super@Outer.count += 5
            return "${super@Outer.count} ${super@Outer.tag} $count"
        }
    }

    fun local(): String {
        class Local : Named {
            override fun name() = "Local"
            fun test() = super@Outer.tag + " " + super<Named>@Outer.name() + " " + super.name()
        }
        return Local().test()
    }

    fun anonymous(): String {
        val o = object : Named {
            override fun name() = "anonymous"
            fun test() = super<Base>@Outer.name() + " " + super.name()
        }
        return o.test()
    }

    fun delegate(): Named = object : Named by super<Base>@Outer.self() {}

    override fun self(): Named = object : Named {}

    fun inLambda(): String {
        val f = { super@Outer.tag + " " + super<Base>@Outer.name() }
        return f()
    }
}

class Builder(val s: String) {
    operator fun get(i: Int): Builder = Builder("$s[$i]")
    operator fun invoke(f: Builder.() -> String): String = f()
}

fun sign(x: Int): String = when { x < 0 -> "negative"; x == 0 -> "zero" else -> "positive" }

fun parity(x: Int) = when (x % 2) { 0 -> { "even" } else -> { "odd" } }

fun main() {
    val o = Outer()
    val i = o.Inner()
    println(i.names())
    println(i.props())
    println(o.count)
    println(o.local())
    println(o.anonymous())
    println(o.delegate().name())
    println(o.inLambda())
    println(Builder("b")[1] { s + "!" })
    println(sign(-3) + " " + sign(0) + " " + sign(7) + " " + parity(3))
}
