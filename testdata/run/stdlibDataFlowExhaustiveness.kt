// A when needn't cover the values that conditions before it rule out
// (Kotlin 2.3's data-flow-based exhaustiveness).
enum class E { A, B, C }

sealed interface S {
    object X : S
    object Y : S
    class Z(val n: Int) : S
}

fun afterReturn(e: E): Int {
    if (e == E.A) return 1
    return when (e) {
        E.B -> 2
        E.C -> 3
    }
}

fun afterTwo(e: E): Int {
    if (e == E.A || e == E.B) return 1
    return when (e) {
        E.C -> 3
    }
}

fun onlyOne(e: E): Int {
    if (e != E.C) return 0
    return when (e) {
        E.C -> 3
    }
}

fun bool(b: Boolean?): Int {
    if ((b == true) == false) return 0
    return when (b) {
        true -> 1
    }
}

fun sealedIs(s: S): Int {
    if (s is S.Z) return s.n
    return when (s) {
        S.X -> 1
        S.Y -> 2
    }
}

fun sealedThrow(s: S): Int {
    (s is S.X) && throw IllegalArgumentException("X")
    return when (s) {
        S.Y -> 2
        is S.Z -> s.n
    }
}

fun sealedRequire(s: S): Int {
    require(s is S.X || s is S.Y)
    return when (s) {
        is S.X -> 1
        is S.Y -> 2
    }
}

fun main() {
    println(E.entries.map { afterReturn(it) })
    println(E.entries.map { afterTwo(it) })
    println(E.entries.map { onlyOne(it) })
    println(listOf(true, false, null).map { bool(it) })
    println(listOf(S.X, S.Y, S.Z(7)).map { sealedIs(it) })
    println(listOf(S.Y, S.Z(8)).map { sealedThrow(it) })
    println(runCatching { sealedThrow(S.X) }.exceptionOrNull()?.message)
    println(listOf(S.X, S.Y).map { sealedRequire(it) })
}
