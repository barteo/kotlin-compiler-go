// Both declarations of a conflicting pair are reported, as kotlinc does.
class B {
    class C // ERROR redeclaration: C
    class C // ERROR redeclaration: class C
}

enum class E {
    FOO, // ERROR redeclaration: FOO
    FOO // ERROR redeclaration: FOO
}

interface CtxA
interface CtxB

context(a: CtxA, b: CtxB) fun byOrder() {} // ERROR conflicting overloads
context(b: CtxB, a: CtxA) fun byOrder() {} // ERROR conflicting overloads

fun test() {
    class Test5(val x: Int) { // ERROR conflicting overloads: Test5
        constructor() : this(0) // ERROR conflicting overloads: Test5
    }
    fun Test5() {} // ERROR conflicting overloads: Test5
    fun Test5(x: Int) = x // ERROR conflicting overloads: Test5
    fun Test5(x: String) = x
}
