class B

class A {
    operator fun B.invoke() {}
    val foo: B.() -> Int get() = { 1 }
    fun m() = 3
}

object O {
    operator fun invoke(x: Int) = x
}

val B.a: () -> Int get() = { 5 }
val f: () -> Int = { 2 }

fun g() = 1

fun test(a: A, b: B) {
    (g)() // ERROR function invocation 'g()' expected
    (a.m)() // ERROR m
    (f)()
    ((f))()
    (O)(1)
    (a.foo)(b)
    (a::m)()
    (::g)()
    with(b) { // ERROR cannot infer
        val y: Int = a()
        (a)() // ERROR cannot be invoked
    }
    with(a) {
        (b.foo)() // ERROR foo
        b.foo()
        (foo)(b)
    }
    with(b) {
        (a.foo)()
    }
}
