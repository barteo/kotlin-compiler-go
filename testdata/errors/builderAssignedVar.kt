// A var a builder's variable types is smart cast by an assignment to the
// value's type and is the builder's variable still, so a member of it
// fixes the variable to the value's type, and a later value of another
// type doesn't fit, as in K2.
class Target { fun member() {} }
class Different

class Buildee<V> {
    fun get(): V = null!!
}

fun <P> build(b: Buildee<P>.() -> Unit): Buildee<P> = Buildee<P>().apply(b)

fun <T> Buildee<T>.apply(b: Buildee<T>.() -> Unit): Buildee<T> {
    b()
    return this
}

fun fixed(): Buildee<Target> = build {
    var v = get()
    v = Target()
    v.member()
    v = Different() // ERROR type mismatch: inferred type is Different but Target was expected
    v = Target()
}

fun unfixed(): Buildee<Any> = build {
    var v = get()
    v = Target()
    v = Different()
}
