import kotlin.reflect.KProperty

class Box<T>(var value: T)
operator fun <T> Box<T>.getValue(thisRef: Any?, property: KProperty<*>): T = value
operator fun <T> Box<T>.setValue(thisRef: Any?, property: KProperty<*>, v: T) { value = v }
fun <T> boxOf(v: T) = Box(v)
inline fun <T> keep(calc: () -> T): T = calc()
fun <T> one(v: T) = listOf(v)

fun main() {
    var m by keep { boxOf(buildMap { put("a", 1) }) }
    m = m + ("b" to 2)
    println(m)
    println(run { one(buildList { add(1); add(2) }) })
    println(keep { one(sequence { yield("s") }) }.map { it.toList() })
}
