// A class's annotations are resolved outside it: they don't see its
// companion's members, and a companion's don't see its own.
annotation class Ann(val c: Int)

@Ann(CONST) // ERROR unresolved reference: CONST
class A {
    @Ann(CONST)
    class B

    @Ann(CONST)
    object O

    companion object {
        const val CONST = 1
    }
}

class C {
    @Ann(inner) // ERROR unresolved reference: inner
    companion object {
        const val inner = 1
    }
}

fun main() {}
