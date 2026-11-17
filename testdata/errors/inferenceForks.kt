// A variable constrained by an intersection of two instances of one
// class forks over them: the first instance whose constraint system
// holds is taken, the cast's type before the declared one.

interface A
class Inv<T>

fun <U> Inv<U>.ext(t: U): U = t
fun <Y> pick(c: Inv<Y>): Y = TODO()
fun <V> two(c: Inv<V>, v: V): V = v

fun <T> declared(t: T, f: Inv<T>): T {
    f as Inv<A>
    return f.ext(t)
}

fun <T> cast(f: Inv<T>, a: A): A {
    f as Inv<A>
    return pick(f)
}

fun <T> neither(f: Inv<T>, s: String) {
    f as Inv<A>
    two(f, s) // ERROR type mismatch
}
