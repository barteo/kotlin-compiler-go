// Nested builders: a call that mentions both builders' variables is
// inferred with the inner one's first, and the outer one's again once
// those are fixed. A fork over a builder's variable, as a smart cast to a
// subclass of another instance makes, waits for all its lambda's
// constraints.

fun <R1> build(block: TI<R1>.() -> Unit): R1 = TODO()
fun <R2> build2(block: TI<R2>.() -> Unit): R2 = TODO()

interface TI<R> {
    fun emit(r: R)
    fun get(): R
}

fun <E> select(vararg x: E): E = x[0]

fun nested(): String = build {
    emit("1")
    build2 {
        emit(1)
        select(this@build.get(), get())
        ""
    }
    ""
}

class Controller<T> {
    fun yield(t: T): Boolean = true
}

fun <S> generate(g: Controller<S>.() -> Unit): S = TODO()

interface A<E>
interface B : A<Int>

fun <F> Controller<F>.baz(a: A<F>) {}

fun <T> forks(a: A<T>, w: T): T = generate {
    if (a is B) {
        baz(a)
    }
    yield(w)
}

// The expected type fixes the variable first.
fun <T> wrongYield(w: T): String = generate {
    yield(w) // ERROR type mismatch
}
