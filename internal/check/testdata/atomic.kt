// The warnings of compare-and-set calls on atomic references of values
// without a consistent identity below, as TestWarningsAtomic reads them:
// each line ends with a comment naming the warnings it reports, if any.
// They follow kotlinc 2.4.21's FirJvmAtomicReferenceToPrimitiveCallChecker
// and FirCommonAtomicReferenceToPrimitiveCallChecker, and the tests
// errorProneAtomic*.kt of its diagnostics tests.
@file:OptIn(kotlin.concurrent.atomics.ExperimentalAtomicApi::class)
@file:Suppress("UNUSED_PARAMETER")

import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference as JRef
import java.util.concurrent.atomic.AtomicReferenceArray
import kotlin.concurrent.atomics.AtomicArray
import kotlin.concurrent.atomics.AtomicReference as KRef

@JvmInline
value class Box(val name: String)

typealias Aliased<T> = JRef<T>

class Sub : JRef<Int>(0)

fun javaRef(r: JRef<Int>, n: JRef<Int?>, l: JRef<Long>, c: JRef<Char>, a: Aliased<Int>, o: JRef<Int?>?) {
    r.compareAndSet(127, 128) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    r.weakCompareAndSetPlain(1, 2) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    r.compareAndExchange(1, 2) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    n.compareAndSet(128, null) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    o?.compareAndSet(null, null) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY
    l.compareAndSet(1L, 2L) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    c.compareAndSet('c', 'd') // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    a.compareAndSet(1, 2) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    r.set(1)
    r.get()
    Sub().compareAndSet(1, 2)
}

fun javaArray(r: AtomicReferenceArray<Long>, s: AtomicReferenceArray<String>) {
    r.compareAndSet(0, 1L, 2L) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    s.compareAndSet(0, "a", "b")
}

fun kotlinRef(r: KRef<Boolean>, a: AtomicArray<Int>) {
    r.compareAndSet(false, true) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    r.compareAndExchange(expectedValue = false, newValue = true) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    a.compareAndSetAt(0, 1, 2) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    r.load()
}

fun valueClass(b: JRef<Box>, any: KRef<Any>, x: Box) {
    b.compareAndSet(x, Box("y")) // ATOMIC_REF_WITHOUT_CONSISTENT_IDENTITY ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    any.compareAndSet(x, Box("y")) // ATOMIC_REF_CALL_ARGUMENT_WITHOUT_CONSISTENT_IDENTITY
    any.compareAndSet("a", "b")
}

fun <T> unaffected(s: JRef<String>, t: JRef<T>, v: T, i: AtomicInteger) {
    s.compareAndSet("a", "b")
    t.compareAndSet(v, v)
    i.compareAndSet(1, 2)
}
