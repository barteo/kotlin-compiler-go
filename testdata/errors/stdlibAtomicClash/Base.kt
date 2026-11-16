// kotlin.concurrent.atomics.AtomicInt is java.util.concurrent.atomic.AtomicInteger
// on the JVM, but another type to Kotlin: a Java method taking the one
// doesn't override a Kotlin member taking the other.
@file:OptIn(kotlin.concurrent.atomics.ExperimentalAtomicApi::class)

import kotlin.concurrent.atomics.AtomicInt

open class Base {
    open fun foo(a: AtomicInt) {}
    open val a: AtomicInt = AtomicInt(0)
}
