fun early(): Int {
    return g() // ERROR unresolved reference: g
    fun g(): Int = 1
}

fun recursive(): Int {
    fun r(n: Int) = if (n == 0) 0 else r(n - 1) // ERROR type checking has run into a recursive problem
    return r(3)
}

fun loops() {
    while (true) {
        fun f() {
            break // ERROR 'break' crosses a function boundary
        }
        f()
    }
}

fun overloads() {
    fun h(x: Int) {} // ERROR conflicting overloads
    fun h(y: Int) {} // ERROR conflicting overloads
}

fun scoped(flag: Boolean): Int {
    if (flag) {
        fun inner(): Int = 1
    }
    return inner() // ERROR unresolved reference: inner
}

fun types(): String {
    fun n(): Int = 1
    fun s(): String = "s"
    val x: String = n() // ERROR type mismatch
    fun bad(): Int {
        return s() // ERROR type mismatch
    }
    return s()
}

fun missing(): Int {
    fun noReturn(): Int { // ERROR missing return statement
    }
    return noReturn()
}

fun modifiers() {
    open fun o() {} // ERROR not applicable to local functions
    o()
}

fun returnFromOuter(): Int {
    fun f(): String {
        return 1 // ERROR type mismatch
    }
    return f().length
}
