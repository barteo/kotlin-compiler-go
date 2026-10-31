// Constructors of inner and local classes: secondary constructors that
// see the outer class's members and the captured locals, a local
// subclass's secondary constructor that passes its superclass's captures,
// super.Inner() and Inner() in a subclass, which construct with this as
// the outer instance, and typealiases of inner classes.

fun <T, R> with(t: T, f: T.() -> R): R = t.f()

open class A(val value: String) {
    inner class B(val s: String) {
        val result = value + "_" + s
    }
}

class C : A("fromC") {
    inner class X : A("fromX") {
        fun own() = B("OK").result
        fun sup() = super.B("OK").result
        fun outer() = super@C.B("OK").result
    }
}

class Outer(val outerProp: String) {
    var log = ""

    inner class In {
        var prop = ""
        init { log += "init;" }
        constructor(x: String) {
            prop = x + "#" + outerProp
            log += "ctor;"
        }
        constructor(x: Int) : this(x.toString() + outerProp) {
            log += "int;"
        }
    }

    inner class Gen<K>(val k: K)
}

typealias OI = Outer.In
typealias OG<K> = Outer.Gen<K>

fun locals(): String {
    var log = ""
    val p = "P"
    open class Base {
        constructor(a: Int) { log += p + a }
    }
    class Sub : Base {
        constructor(x: Int) : super(x) { log += "Sub" }
        constructor() : this(7) { log += "()" }
    }
    Sub()
    Sub(1)
    return log
}

fun main() {
    val x = C().X()
    println(x.own())
    println(x.sup())
    println(x.outer())

    val o = Outer("o")
    println(o.In(5).prop)
    println(o.log)
    println(o.OI("alias").prop)
    println(o.OG<Int>(3).k)
    println(with(o) { OG<String>("w").k })
    println(locals())
}
