// As in K2, a protected override is the override's class's: A's code
// can't call it on a B, though it overrides A's member, but can through a
// smart cast from a type whose member A's code sees, which K2 falls back
// to.
open class A {
    protected open fun f() = 1
    protected open val v: Int get() = 1
    val held: A = B()

    fun t1(a: A, b: B, c: C) {
        if (a is B) { a.f(); a.v }
        if (held is B) { held.f(); held.v }
        b.f() // ERROR cannot access 'f': it is protected in 'B'
        b.v // ERROR cannot access 'v': it is protected in 'B'
        c.f() // ERROR cannot access 'f': it is protected in 'B'
        val d = a as? B ?: return
        d.f() // ERROR cannot access 'f': it is protected in 'B'
    }

    fun t2(a: Any, b: B) {
        if (a is B) a.f() // ERROR cannot access 'f': it is protected in 'B'
        (b as A).f()
    }

    fun t3(e: E, xs: List<B>) {
        e.f()
        xs[0].f() // ERROR cannot access 'f': it is protected in 'B'
        xs.first().v // ERROR cannot access 'v': it is protected in 'B'
    }

    class N {
        fun t(b: B, a: A) {
            b.f() // ERROR cannot access 'f': it is protected in 'B'
            if (a is B) a.f()
        }
    }
}

open class B : A() {
    override fun f() = 2
    override val v: Int get() = 2
}

class C : B()

class E : A() {
    fun t(b: B, e: E) {
        b.f() // ERROR cannot access 'f': it is protected in 'B'
        e.f()
    }
}
