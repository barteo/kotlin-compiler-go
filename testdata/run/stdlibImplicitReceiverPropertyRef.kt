// ::p for a property of a lambda's implicit receiver is that receiver's
// this::p.

import kotlin.reflect.KMutableProperty0
import kotlin.reflect.KProperty0

class Foo {
    var a: Int = 1
    val b: String = "b"
}

fun set100(p: KMutableProperty0<Int>) = p.set(100)
fun name(p: KProperty0<*>) = p.name + "=" + p.get()

fun main() {
    val f = Foo()
    with(f) {
        set100(::a)
        println(name(::a))
        println(name(::b))
    }
    f.apply { a = 7; println(name(::a)) }
    println(f.a)
}
