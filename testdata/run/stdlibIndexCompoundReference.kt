// A callable reference or lambda as the index of a compound assignment
// takes its type from the operator get's parameter: A[::f] += 1 converts
// ::f, a vararg function, to the fun interface.
fun interface IFoo {
    fun foo(i: Int)
}

object A {
    var stored = 10
}

operator fun A.get(i: IFoo): Int {
    i.foo(1)
    return stored
}
operator fun A.set(i: IFoo, newValue: Int) {
    stored = newValue
}

fun withVararg(vararg xs: Int) = 42
fun plain(x: Int) {
    println("plain $x")
}

fun main() {
    A[::withVararg] += 1
    println(A.stored)
    A[::plain] += 2
    A[{ x -> println("lambda $x") }] *= 2
    println(A.stored)
}
