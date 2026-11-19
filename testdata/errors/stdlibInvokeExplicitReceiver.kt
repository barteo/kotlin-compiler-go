// A property of a function type with a receiver, B.() -> Unit, read from
// a written receiver, a.foo(), is invoked with its receiver passed:
// a.foo(b). An implicit B isn't passed, as it is to (a.foo)() and to foo()
// on an implicit receiver. x.bar() for a B x, in scope of a B's bar, is
// that bar invoked with x.
class A {
    val foo: B.() -> Unit get() = null!!
    fun inA(b: B) { with(b) { foo() } }
}

class B {
    val bar: B.() -> Unit get() = null!!
}

val A.ext: B.() -> Unit get() = null!!
val top: B.() -> Unit get() = null!!

fun test(a: A, b: B, b2: B) {
    with(b) {
        a.foo() // ERROR invoke
        a.foo(this)
        (a.foo)()
        (a.foo)(this)
        a.ext() // ERROR invoke
        a.ext(b)
        top()
        bar()
        b.bar()
        b2.bar()
        this.bar()
        with(a) { foo() }
    }
    a.foo(b)
    a.foo() // ERROR invoke
    b.(a.foo)()
}

fun B.test2(a: A) {
    a.foo() // ERROR invoke
    (a.foo)()
}
