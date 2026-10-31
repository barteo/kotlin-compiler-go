// The arguments of an inner class's super(...) and this(...) calls, and
// its constructors' default values, see the outer class's instance and
// the class's nested classes, but not the class's own members.

open class Base(val callback: () -> String)

open class Named(val name: String)

class Outer(val o: String) {
    val s = "S"

    inner class InnerArg(val i: String) {
        val result: String get() = o + i
    }

    inner class WithDefault(val i: InnerArg = InnerArg("B")) {
        fun foo() = i.result
    }

    inner class Captures : Base({ s + o })

    inner class Secondary : Named {
        constructor(x: Int) : super(o + x)
        constructor() : this(prefix().length)
    }

    fun prefix() = "prefix"

    class Nested(val k: Kind = Kind()) {
        class Kind {
            override fun toString() = "Kind"
        }
    }
}

fun main() {
    val outer = Outer("A")
    println(outer.WithDefault().foo())
    println(outer.Captures().callback())
    println(outer.Secondary().name)
    println(Outer.Nested().k)
}
