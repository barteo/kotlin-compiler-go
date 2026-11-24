package p

// A member of a class the code can't see is invisible, though public:
// another file's private class's, reached through a value of its type
// that a function exposes, or a private nested enum's entry, imported.

private open class A {
    fun bar() {}
    val p = 1
}

fun makeA() = A() // ERROR exposes its 'private-in-file' return type

private class Outer {
    private enum class Nested { Entry }

    fun inside() = Nested.Entry.ordinal + makeA().p
}

fun sameFile() {
    makeA().bar()
    makeA().p
}

// A protected nested class's members are visible where they are
// inherited, not through a value outside the outer class's subclasses.
open class Holder {
    protected open class Prot {
        fun prot() {}
    }

    open class Sub {
        protected open class Deeper : Prot() // ERROR exposes its 'protected' supertype 'Holder.Prot'
    }
}

class Inherits : Holder.Sub() {
    class Leaf : Holder.Sub.Deeper() { // ERROR exposes its 'protected' supertype 'Holder.Sub.Deeper'
        init {
            prot()
        }
    }
}
