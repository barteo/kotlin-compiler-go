// A delegated property declared in a builder's lambda calls its
// delegate's provideDelegate operator each time the lambda is checked:
// with the builder's variables unfixed, and again once they are.

import kotlin.reflect.KProperty

class Buildee<B> {
    fun processCallable(arg: () -> B): () -> B = arg
}

fun <T> build(instructions: Buildee<T>.() -> Unit): Buildee<T> = Buildee<T>().apply(instructions)

class Delegate<D>(private val value: D) {
    operator fun getValue(thisRef: Any?, property: KProperty<*>) = value
}

operator fun <P> (() -> P).provideDelegate(thisRef: Any?, property: KProperty<*>) = Delegate(this())

class K {
    private val member = build {
        object {
            val bar by processCallable(::foo)
            private val foo get() = ""
            operator fun <Q> (() -> Q).provideDelegate(thisRef: Any?, property: KProperty<*>) = Inner(this())
            inner class Inner<I>(private val value: I) {
                operator fun getValue(thisRef: Any?, property: KProperty<*>) = value
            }
        }
    }
}

fun local(): Buildee<String> = build {
    val bar by processCallable { "" }
    val n: Int = bar // ERROR type mismatch: inferred type is String but Int was expected
}
