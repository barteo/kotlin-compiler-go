// The warnings of the tailrec functions below, as TestWarningsTailrec
// reads them: each line ends with a comment naming the warnings it
// reports, if any. They are those kotlinc 2.4.21's diagnostics tests
// tailRec*.kt expect of the same code.
@file:Suppress("UNUSED_EXPRESSION", "UNUSED_VARIABLE", "UNREACHABLE_CODE")
tailrec fun foo1() { // NO_TAIL_CALLS_FOUND
    foo1() // NON_TAIL_RECURSIVE_CALL
    1
}
tailrec fun foo4() {
    if (true) foo4()
    else foo1()
}
tailrec fun foo5() {
    return foo5()
    foo4()
}
tailrec fun foo6(b: Boolean) { // NO_TAIL_CALLS_FOUND
    while (b) {
        foo6(!b) // NON_TAIL_RECURSIVE_CALL
    }
}
tailrec fun foo7() {
    while (true) {
        foo7()
        return
    }
}
tailrec fun foo8() { // NO_TAIL_CALLS_FOUND
    while (true) {
        foo8() // NON_TAIL_RECURSIVE_CALL
        throw Exception()
    }
    foo8()
}
tailrec fun foo11(): String { // NO_TAIL_CALLS_FOUND
    return "hello ${foo11()}" // NON_TAIL_RECURSIVE_CALL
}
tailrec fun nested() { // NO_TAIL_CALLS_FOUND
    run {
        nested() // NON_TAIL_RECURSIVE_CALL
    }
    fun bar() {
        nested() // NON_TAIL_RECURSIVE_CALL
    }
    bar()
}
tailrec fun inRun() {
    run {
        return inRun()
    }
}
tailrec fun inTry() { // NO_TAIL_CALLS_FOUND
    try {
        inTry() // TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED
    } catch (e: Exception) {
        inTry() // TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED
    } finally {
        inTry() // TAIL_RECURSION_IN_TRY_IS_NOT_SUPPORTED
    }
}
tailrec fun foo5(param: Int) {
    if (true) {
        return foo5(param)
    } else {
        try { } finally { }
    }
}
tailrec fun foo4(counter: Int): Int? {
    if (counter < 0) return null
    if (counter == 0) return 777
    return foo4(-1) ?: foo4(counter - 1) // NON_TAIL_RECURSIVE_CALL
}
tailrec fun Int.ext(): Int {
    return boo()?.ext() ?: 1
}
fun Int.boo(): Int? = this + 1
class A(val a: A) {
    tailrec fun other() { // NO_TAIL_CALLS_FOUND
        a.other() // NON_TAIL_RECURSIVE_CALL
    }
    tailrec fun self() {
        this.self()
    }
    tailrec fun implicit() {
        implicit()
    }
    tailrec fun with() { // NO_TAIL_CALLS_FOUND
        with(a) {
            with() // NON_TAIL_RECURSIVE_CALL
            return
        }
    }
}
object O {
    tailrec fun obj(n: Int): Int = if (n == 0) 0 else O.obj(n - 1)
}
