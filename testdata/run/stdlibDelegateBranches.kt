import kotlin.reflect.KProperty
data class Ref<D>(val t: D)
operator fun <V> Ref<V>.getValue(hisRef: Any?, property: KProperty<*>): V = this.t
@Suppress("UNCHECKED_CAST")
fun <E> List<Ref<*>>.getElement(i: Int): Ref<E> = this[i] as Ref<E>
fun test(list: List<Ref<*>>, arg: Boolean) {
    val a: String by if (arg) list.getElement(0) else list.getElement(1)
    val b: String by list.getElement(1) ?: error("")
    val c: String by try { list.getElement(0) } finally {}
    println(a + b + c)
}
fun main() { test(listOf(Ref("x"), Ref("y")), false) }
