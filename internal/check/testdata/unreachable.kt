// The warnings kotlinc 2.4.21 reports with -Wextra of the unreachable
// code below, as TestWarningsUnreachable reads them: each line ends with
// a comment naming the warnings it reports, if any.
fun todo(): Nothing = throw Exception()
fun bar(a: Any? = null) {}
fun bar2(a: Int) {}
fun baz(a: Any?) {}
fun <T> gen(a: T) {}
fun u1_a() {
    return
    bar() // UNREACHABLE_CODE
    bar() // UNREACHABLE_CODE
    if (true) { // UNREACHABLE_CODE
        bar() // UNREACHABLE_CODE
    }
}
fun u1_b(x: Int) {
    val y = todo() // UNREACHABLE_CODE UNUSED_VARIABLE
    bar(); bar() // UNREACHABLE_CODE
}
fun u1_c(x: Int) {
    bar(todo()) // UNREACHABLE_CODE
    bar() // UNREACHABLE_CODE
}
fun u1_d(x: Int) {
    if (x > 1) return else throw Exception()
    bar() // UNREACHABLE_CODE
}
fun u1_e(x: Int): Int {
    while (true) { }
    bar() // UNREACHABLE_CODE
}
fun u1_f(x: Int) {
    for (i in 1..2) {
        continue
        bar() // UNREACHABLE_CODE
    }
}
fun u1_h(x: Int) {
    val l = l@{ return@l; bar() } // UNREACHABLE_CODE
    l()
}

fun u2_a1(x: Int) {
    run { return }
}
fun u2_a2(x: Int) {
    run {
        return
    }
}
fun u2_a3(x: Int) {
    if (todo()) bar() else bar() // UNREACHABLE_CODE
}
fun u2_a4(x: Int) {
    return todo() // UNREACHABLE_CODE
}
fun u2_a5(x: Int) {
    throw todo() // UNREACHABLE_CODE
}
fun u2_a6(x: Int?) {
    val y = x ?: return
    bar(y)
}
fun u2_a7(x: Int) {
    val s = "a${todo()}" // UNREACHABLE_CODE UNUSED_VARIABLE
}
fun u2_a8(x: Int) {
    todo().hashCode() // UNREACHABLE_CODE
}
fun u2_a9(x: Int) {
    bar( // UNREACHABLE_CODE
        todo()
    )
}
fun u2_a10(x: Int) {
    var z = 1 // VARIABLE_NEVER_READ
    z = todo() // UNREACHABLE_CODE ASSIGNED_VALUE_IS_NEVER_READ
}
fun u2_a11(x: Int) {
    if (x > 0) {
        return
    } else {
        todo()
    }
}
fun u2_a12(x: Int) {
    when (x) {
        1 -> return
        else -> throw Exception()
    }
    bar() // UNREACHABLE_CODE
}
fun u2_a13(x: Int) {
    try { return } finally { bar() }
    bar() // UNREACHABLE_CODE
}
fun u2_a14(x: Int) {
    try { todo() } catch (e: Exception) { return }
    bar() // UNREACHABLE_CODE
}
fun u2_a15(x: Int) {
    val q = listOf(1).map { todo() }
    bar(q)
}
fun u2_a16(x: Int) {
    return
    when (x) { // UNREACHABLE_CODE
        1 -> bar() // UNREACHABLE_CODE
        else -> bar() // UNREACHABLE_CODE
    }
    for (i in 1..2) { // UNREACHABLE_CODE
        bar()
    }
    while (x > 0) { // UNREACHABLE_CODE
        bar()
    }
    try { // UNREACHABLE_CODE
        bar()
    } finally {
        bar()
    }
    do { // UNREACHABLE_CODE
        bar()
    } while (x > 0)
    if (x > 0) // UNREACHABLE_CODE
        bar() // UNREACHABLE_CODE
    else
        bar() // UNREACHABLE_CODE
    val l = { // UNREACHABLE_CODE UNUSED_VARIABLE
        bar()
    }
    fun loc() { // UNREACHABLE_CODE
        bar()
    }
    bar(1 + // UNREACHABLE_CODE
        2)
}

fun u3_foo(x: String) {
    x as Nothing // CAST_NEVER_SUCCEEDS
}
fun u3_bar() {
    val y: Nothing = null!! // UNREACHABLE_CODE UNUSED_VARIABLE
}

fun u5_a1() { val y: Int = null!! } // UNUSED_VARIABLE
fun u5_a2() { val y = null!! } // UNREACHABLE_CODE UNUSED_VARIABLE
fun u5_a3() { bar2(null!!) } // UNREACHABLE_CODE
fun u5_a4(x: String) { bar2(x as Nothing) } // CAST_NEVER_SUCCEEDS
fun u5_a5(x: String) { val z: Int = x as Nothing } // CAST_NEVER_SUCCEEDS UNUSED_VARIABLE
fun u5_a6() { baz(null!!) } // UNREACHABLE_CODE
fun u5_a7() { gen(null!!) } // UNREACHABLE_CODE
fun u5_a8() { val y: Int = todo() } // UNREACHABLE_CODE UNUSED_VARIABLE
fun u5_a9(x: String?) { val y: Int = x!! as Nothing } // CAST_NEVER_SUCCEEDS UNUSED_VARIABLE
fun u5_a10() { val y: String = "a" + null!! } // UNREACHABLE_CODE UNUSED_VARIABLE
fun u5_a11() { var y = 1; y = null!! } // ASSIGNED_VALUE_IS_NEVER_READ VARIABLE_NEVER_READ
fun u5_a12() { listOf(1)[null!!] } // UNREACHABLE_CODE

fun u6_a1() {
    val y: Int = null!! // UNUSED_VARIABLE
    bar()
}
fun u6_a2(x: String) {
    x as Nothing // CAST_NEVER_SUCCEEDS
    bar()
}
fun u6_a3(x: String) {
    val y = x as Nothing // CAST_NEVER_SUCCEEDS UNUSED_VARIABLE
    bar()
}
fun u6_a4() {
    var y = 1 // VARIABLE_NEVER_READ
    y = null!! // ASSIGNED_VALUE_IS_NEVER_READ
    bar()
}
fun u6_a5(): Int {
    return null!!
    bar() // UNREACHABLE_CODE
}
fun u6_a6(x: String?) {
    val y: String = x ?: null!! // UNUSED_VARIABLE
    bar()
}
fun u6_a7() {
    val y: Int = if (true) null!! else null!! // UNREACHABLE_CODE UNUSED_VARIABLE
    bar() // UNREACHABLE_CODE
}
fun u6_a8() {
    null!!
    bar() // UNREACHABLE_CODE
}
fun dw1() {
    do {
    } while (todo())
    bar() // UNREACHABLE_CODE
}
fun dw2(b: Boolean) {
    do {
        if (b) break
    } while (todo())
    bar()
}
fun dw3(b: Boolean) {
    do {
        if (b) continue
    } while (todo())
    bar() // UNREACHABLE_CODE
}
fun dw4(): Int {
    do { } while (todo())
}
fun dw5(b: Boolean) {
    do {
        while (b) { break }
    } while (todo())
    bar() // UNREACHABLE_CODE
}
