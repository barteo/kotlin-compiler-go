// A builder's variable that nothing infers is reported at the builder's
// call, and so is the variable of a call in its lambda that only the
// builder's bounds.

class Foo<K>

fun <K> buildFoo(builderAction: Foo<K>.() -> Unit): Foo<K> = Foo()

fun <K> Foo<K>.bar(x: Int = 1) {}

interface FlowCollector<in T>

interface Flow<out T>

fun <T> flowOf(value: T): Flow<T> = TODO()

fun <T, R> Flow<T>.transform(transform: FlowCollector<R>.(T) -> Unit): Flow<R> = TODO()

fun <T> doEmit(collector: FlowCollector<T>) {}

fun main() {
    val x = buildFoo { // ERROR cannot infer type for type parameter 'K'
        bar() // ERROR cannot infer type for type parameter 'K'
    }
    flowOf(1).transform { // ERROR cannot infer type for type parameter 'R'
        doEmit(this) // ERROR cannot infer type for type parameter 'T'
    }
}
