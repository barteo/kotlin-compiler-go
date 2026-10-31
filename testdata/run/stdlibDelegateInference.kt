// A delegate's type argument is inferred from the property's declared
// type, also through an extension getValue and through a function that
// returns what its lambda does, as Compose's
// var x: T? by remember { mutableStateOf(null) }.
import kotlin.reflect.KProperty

class Box<T>(var value: T)

operator fun <T> Box<T>.getValue(thisRef: Any?, property: KProperty<*>): T = value

operator fun <T> Box<T>.setValue(thisRef: Any?, property: KProperty<*>, v: T) {
    value = v
}

fun <T> boxOf(v: T): Box<T> = Box(v)

inline fun <T> keep(calc: () -> T): T = calc()

fun <T> keep(key: Any?, calc: () -> T): T = calc()

class Holder {
    var name: String? by keep { boxOf(null) }
}

fun main() {
    var a: String? by boxOf(null)
    println(a)
    a = "a"
    println(a)
    var b: List<Int>? by keep { boxOf(null) }
    b = listOf(1, 2)
    println(b)
    var c: Int? by keep(1) { boxOf(null) }
    c = 3
    println(c)
    val h = Holder()
    h.name = "h"
    println(h.name)
}
