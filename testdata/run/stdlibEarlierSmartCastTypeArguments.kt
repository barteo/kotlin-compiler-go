// A val keeps its earlier smart casts: after ref as Function2<...> and a
// later smart cast to a suspend function type, is can still check
// Function2<...>, whose type arguments the first cast gives.

import kotlin.coroutines.Continuation

fun check(ref: suspend (Int) -> Int): Boolean {
    ref as Function2<Int, Continuation<Int>, Any?>
    if (ref is suspend (Int) -> Any?) {
        return ref is Function2<Int, Continuation<Int>, Any?>
    }
    return false
}

fun main() {
    println(check { it + 1 })
}
