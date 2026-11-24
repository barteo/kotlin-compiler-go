// The warnings kotlinc 2.4.21 reports of x.f() that may invoke a property
// f whose type is omitted, as TestWarningsOrderDependent reads them: the
// property is being inferred, or its inference runs into a cycle, so the
// result depends on the order of inference (KT-76240). Its own
// initializer, val d = O1.d(), is a loop of one, which isn't reported.
fun Int.f(): String = "ext func"
fun String.g(): Boolean = false
fun Int.h(): String = ""

class Foo {
    val f = f()
    fun f() = 42.f() // IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT
}

class Bar {
    fun g() = "s2".g() // IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT
    val g = g() // error
}

class Loop(val b1: Boolean, val b2: Boolean) {
    fun k() = h()
    fun h() = if (b1) 42.h() else true // IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT
    val h = if (b2) h() else false // error
}

object O1 {
    fun d(): Any = Any()
}

object O2 {
    val d = O1.d()
}

class Typed {
    val f5: Int.() -> Unit = {}
    val g5 = f5
    val x = 42.g5()
}

fun Int.g5(): String = ""
