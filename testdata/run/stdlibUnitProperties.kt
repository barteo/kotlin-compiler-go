// Properties of type Unit: as with kotlinc, their fields, getters and
// setters hold the kotlin.Unit object, though a data class's component
// returns void.
import kotlin.reflect.KProperty

var log = ""

fun act(s: String) {
    log += s
}

val b = act("b")
val c = act("c")
var top: Unit = Unit
val custom: Unit get() = act("g")

class D {
    operator fun getValue(a: Any?, p: KProperty<*>) = act("d")
    operator fun setValue(a: Any?, p: KProperty<*>, v: Unit) = act("s")
}

var delegated by D()

interface I {
    val u: Unit
}

open class Base : I {
    override val u = act("u")
    var w: Unit = Unit
    private var p = Unit
    open val o: Unit get() = Unit

    fun swap(): Unit {
        p = w
        return p
    }
}

class Derived : Base() {
    override val o: Unit get() = super.o
}

object O {
    val x = Unit
    var y = act("o")
}

class Holder {
    companion object {
        var z: Unit = Unit
    }
}

data class A(val x: Unit, val n: Int)

fun main() {
    println("$b $c $top $log")
    top = act("t")
    println("$top $custom $delegated $log")
    delegated = Unit
    val i: I = Base()
    println(i.u)
    val base = Derived()
    base.w = Unit
    println("${base.w} ${base.swap()} ${base.o}")
    println("${O.x} ${O.y}")
    O.y = Unit
    Holder.z = O.y
    println(Holder.z == Unit)
    val a = A(Unit, 1)
    println(a)
    println("${a.component1()} ${a.x} ${a.copy(n = 2)} ${a == A(Unit, 1)} ${a.hashCode() == A(Unit, 1).hashCode()}")
    val (x, n) = a
    println("$x $n")
    println(log)
}
