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

class Props {
    val a = 1 // ERROR conflicting declarations: a
    val a = "" // ERROR conflicting declarations: a
}

enum class F {
    BAR; // ERROR redeclaration: BAR

    val BAR = 1 // ERROR redeclaration: BAR
}

val top = 1 // ERROR conflicting declarations: top
val top = "" // ERROR conflicting declarations: top

fun locals() {
    val x = 1 // ERROR conflicting declarations: x
    val x = "" // ERROR conflicting declarations: x
}

fun params(p: Int, p: String) {} // ERROR conflicting declarations: p

context(c: CtxA) // ERROR conflicting declarations: c
fun ctxAndParam(c: Int) {} // ERROR conflicting declarations: c

interface Sus {
    suspend fun s() // ERROR conflicting overloads: s
    fun s() // ERROR conflicting overloads: s
}

open class Base(x: Int)

class NoPrimary : Base(1) { // ERROR supertype initialization is impossible without a primary constructor
    constructor() : super(2) // ERROR conflicting overloads: constructor NoPrimary()
}
