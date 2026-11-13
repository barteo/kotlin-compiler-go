package lib

inline fun <reified T> whenIs(value: Any?, block: (t: T) -> Unit) {
    when (value) {
        is T -> block(value)
        else -> Unit
    }
}

inline fun twice(block: () -> Unit) {
    block()
    block()
}

class Holder<S>(val value: S)

// Inline functions that pass their lambda parameter on to another inline
// function only invoke it, so their own callers' lambdas are inlined,
// also those that are another inline function's parameter in turn.
abstract class Base<S>(initial: S) {
    val state = Holder(initial)

    inline fun <reified T> whenState(crossinline block: (t: T) -> Unit) = whenIs(state.value, block)

    inline fun repeated(label: String, block: () -> Unit) {
        println(label)
        twice(block)
    }
}
