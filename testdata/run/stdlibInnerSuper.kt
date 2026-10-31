// Classes that extend an inner class without being inner classes of its
// outer class: object expressions, local classes and a local class in an
// inner class. Their super(...) call passes the innermost implicit
// receiver that is an instance of the outer class.

open class Outer(val name: String) {
    open inner class Inner(val d: Double = -1.0, val s: String = "s") {
        open fun result() = "Inner of $name"
    }

    // An object expression in an initializer, with default arguments.
    val obj = object : Inner(s = "named") {
        override fun result() = s + " " + name
    }

    fun local(): Inner {
        class Local : Inner(2.0) {
            override fun result() = "Local in $name, $d"
        }
        return Local()
    }

    // A with receiver is the innermost instance of Outer.
    fun viaWith(): String = with(Outer(name + "+")) {
        object : Inner() {}.result()
    }

    inner class Child(n: String) : Outer(n) {
        fun test(): Inner {
            class InChild : Inner() {}
            return InChild()
        }
    }
}

fun main() {
    val o = Outer("O")
    println(o.obj.result())
    println(o.local().result())
    println(o.viaWith())
    println(o.Child("child").test().result())
}
