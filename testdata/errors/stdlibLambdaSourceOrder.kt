// As kotlinc fixes the variables of lambdas' parameters in the order the
// lambdas are written, a lambda passed before a call whose own lambda
// takes Map.Entry<K, V> is analyzed first (KT-68570): its T : Any, which
// only the later lambda's result would infer, is its bound Any. A T fixed
// from map's (T) -> R receiver, from filter's V, or one without a bound
// waits for the later lambda.

class Foo
class Bar {
    fun barFunction() = 1
}

fun <T : Any> takeMap(map: Map<Foo, T>, doSomething: (T) -> Unit) {}
fun <T : Any> takeMap2(doSomething: (T) -> Unit, map: Map<Foo, T>) {}
fun <T> unbounded(doSomething: (T) -> Unit, map: Map<Foo, T>) {}
fun <T : Any> takeList(doSomething: (T) -> Unit, list: List<T>) {}

fun main(map: Map<Foo, Bar>, bars: List<Bar>) {
    takeMap(map.mapValues { it.value }) { it.barFunction() }
    takeMap(doSomething = { it.barFunction() }, map = map.mapValues { it.value }) // ERROR unresolved reference: barFunction
    takeMap2({ it.barFunction() }, map.mapValues { it.value }) // ERROR unresolved reference: barFunction

    takeMap2({ it.barFunction() }, map)
    takeMap2({ it.barFunction() }, map.filter { true })
    takeMap2({ b: Bar -> b.barFunction() }, map.mapValues { it.value })
    unbounded({ it.barFunction() }, map.mapValues { it.value })
    takeList({ it.barFunction() }, bars.map { it })
}
