suspend fun one(): Int = 1

fun plain() {
    one() // ERROR suspend function 'one' can only be called from a coroutine or another suspend function
}

fun lambdas() {
    val f = { one() } // ERROR can only be called from a coroutine
    listOf(1).map { one() } // ERROR can only be called from a coroutine
    f()
}

suspend fun inlined() {
    listOf(1).forEach { one() }
    val s: suspend () -> Int = { one() }
    s()
}

suspend fun notInlined() {
    Thread { one() } // ERROR can only be called from a coroutine
}

open class A {
    open suspend fun f() {}
    open fun g() {}
}

class B : A() {
    override fun f() {} // ERROR non-suspend function 'f' cannot override suspend function 'f' defined in 'A'
    override suspend fun g() {} // ERROR suspend function 'g' cannot override non-suspend function 'g' defined in 'A'
}

fun restricted(): Sequence<Int> = sequence {
    yield(1)
    one() // ERROR restricted suspending functions can invoke member or extension suspending functions only on their restricted coroutine scope
}

class Lock {
    suspend fun critical() {
        synchronized(this) {
            one() // ERROR the 'one' suspension point is inside a critical section
        }
    }
}

val toPlain: () -> Int = suspend { 1 } // ERROR type mismatch
val toSuspend: suspend () -> Int = toPlain // ERROR type mismatch

fun local() {
    suspend fun inner() = one()
    inner() // ERROR suspend function 'inner' can only be called from a coroutine
}

interface Unresolved { suspend fun f(x: Missing) } // ERROR unresolved or unsupported type: Missing
class ImplementsUnresolved : Unresolved { override suspend fun f(x: Missing) {} } // ERROR unresolved or unsupported type: Missing

fun suspendCheck(x: Function<*>): Boolean = x is suspend () -> Unit // ERROR cannot check for instance of erased type
fun suspendCheckKnown(x: suspend (Int) -> Int): Boolean = x is suspend (Int) -> Int
