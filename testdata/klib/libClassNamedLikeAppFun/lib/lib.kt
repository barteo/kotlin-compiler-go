package m

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.contract

data class Items(val own: List<Int> = emptyList(), val other: List<Int> = emptyList())

sealed class Node {
    data class Leaf(val id: Int) : Node()
    object Empty : Node()
}

@OptIn(ExperimentalContracts::class)
inline fun <reified T> expectIs(v: Any?): T {
    contract { returns() implies (v is T) }
    if (v !is T) throw IllegalStateException("not a ${T::class.simpleName}")
    return v
}
