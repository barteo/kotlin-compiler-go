@file:OptIn(kotlin.concurrent.atomics.ExperimentalAtomicApi::class)

import kotlin.concurrent.atomics.AtomicInt

class FakeOverride : JavaChild() // ERROR inherited platform declarations clash

class ExplicitOverride : JavaChild() {
    override fun foo(a: AtomicInt) {} // ERROR accidental override
    override val a: AtomicInt = AtomicInt(0) // ERROR accidental override
}

@OptIn(kotlin.concurrent.atomics.ExperimentalAtomicApi::class)
fun load(x: AtomicInt): Int = x.load()
