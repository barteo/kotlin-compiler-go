// The warnings kotlinc 2.4.21 reports with -Wextra of the local variables
// below, as TestWarningsUnusedVar reads them: each line ends with a
// comment naming the warnings it reports, if any.
import kotlin.reflect.KProperty
fun use(x: Any?) {}
fun <T> run2(f: () -> T): T = f()
object Dg { operator fun getValue(t: Any?, p: KProperty<*>) = 1; operator fun setValue(t: Any?, p: KProperty<*>, v: Int) {} }
fun a1(c: Boolean, p: Pair<Int, Int>) {
    val a = 1 // UNUSED_VARIABLE
    val b: Int // UNUSED_VARIABLE
    var d = 2 // VARIABLE_NEVER_READ
    d = 3 // ASSIGNED_VALUE_IS_NEVER_READ
    val (e, f) = p // UNUSED_VARIABLE
    use(e)
    val (_, g) = p // UNUSED_VARIABLE
    lateinit var h: String // UNUSED_VARIABLE
    val i by Dg // UNUSED_VARIABLE
    var j by Dg
    j = 2
    when (val k = 3) { else -> use(1) } // UNUSED_VARIABLE
    try { use(1) } catch (ex: Exception) { }
    for (l in 0..1) { }
    val m = 1
    run2 { m }
    val n = 1
    fun lf() = n
    lf()
}
fun a2(c: Boolean) {
    var a = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    a = 2
    use(a)
    var b = 1
    use(b)
    b = 2 // ASSIGNED_VALUE_IS_NEVER_READ
    var d = 1
    if (c) d = 2
    use(d)
    var e = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    if (c) { e = 2 } else { e = 3 }
    use(e)
    var f = 1
    f = f + 1
    use(f)
    var g = 1
    g++ // ASSIGNED_VALUE_IS_NEVER_READ
    var h = 1
    use(h++) // ASSIGNED_VALUE_IS_NEVER_READ
    var i = 1
    use(++i)
    var j = 1
    j += 1 // ASSIGNED_VALUE_IS_NEVER_READ
    var k = 1
    if (c) { k = 2; return } // ASSIGNED_VALUE_IS_NEVER_READ
    use(k)
}
fun a3(c: Boolean) {
    var a = 1
    use(a)
    while (c) { a = 2; a = 3; use(a) } // ASSIGNED_VALUE_IS_NEVER_READ
    var b = 1
    try { b = 2; use(1); b = 3 } catch (ex: Exception) { use(b) }
    var d = 1
    run2 { d = 2 }
    use(d)
    var e = 1
    run2 { use(e) }
    e = 2
    var f = 1
    while (c) { f = 2; if (c) break; f = 3 }
    use(f)
    var h = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    h = 2 // ASSIGNED_VALUE_IS_NEVER_READ
    if (c) return
    h = 3
    use(h)
    var i = 1
    use(i)
    i = 2 // ASSIGNED_VALUE_IS_NEVER_READ
    if (c) { i = 3 } // ASSIGNED_VALUE_IS_NEVER_READ
}
class Cl {
    init { val a = 1 } // UNUSED_VARIABLE
    val p: Int get() { val q = 1; return 2 } // UNUSED_VARIABLE
    fun m() { var r = 1; r = 2; use(r) // VARIABLE_INITIALIZER_IS_REDUNDANT
        val s = object { // UNUSED_VARIABLE
            fun t() { val u = 1 } // UNUSED_VARIABLE
        }
    }
}
fun a4(c: Boolean) {
    var a = 1
    use(a)
    a = 2 // ASSIGNED_VALUE_IS_NEVER_READ
    var b = 1
    use(b)
    b = 2
    use(b)
    when { c -> b = 3 } // ASSIGNED_VALUE_IS_NEVER_READ
    var d: Int // VARIABLE_NEVER_READ
    d = 1 // ASSIGNED_VALUE_IS_NEVER_READ
    var e: Int //
    e = 1
    use(e)
    var f = 1
    use(f)
    f = 2; f = 3 // ASSIGNED_VALUE_IS_NEVER_READ
    use(f)
}
// Captured variables: kotlinc judges their writes before the first
// lambda, local function or local class that uses them.
fun later3(block: () -> Unit) {}
inline fun inl3(block: () -> Unit) { block() }
interface I3 { fun f() }

fun cap1(c: Boolean) {
    var x = 1
    use(x)
    while (c) {
        x = 2
        x = 3
        later3 { use(x) }
    }
}
fun cap2(c: Boolean) {
    var x = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    x = 2 // ASSIGNED_VALUE_IS_NEVER_READ
    if (c) {
        x = 3
        later3 { use(x) }
    } else {
        x = 4
    }
    use(x)
}
fun cap3(c: Boolean) {
    var x = 1
    if (c) {
        x = 2 // ASSIGNED_VALUE_IS_NEVER_READ
        x = 3
    }
    val o = object : I3 { override fun f() { use(x) } }
    o.f()
}
fun cap4() {
    var x = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    x = 2
    class L { fun g() = x }
    L().g()
}
fun cap5() {
    var x = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    x = 2
    inl3 { use(x) }
    x = 3
    use(x)
}
fun cap6() {
    var x = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    later3 { x = 4 }
    x = 2
    use(x)
}
fun cap7(c: Boolean) {
    var x = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    when {
        c -> { x = 2; x = 3 } // ASSIGNED_VALUE_IS_NEVER_READ
        else -> x = 4
    }
    later3 { use(x) }
}
fun cap8() {
    var x = 1
    x++ // ASSIGNED_VALUE_IS_NEVER_READ
    x = 5
    later3 { use(x) }
}
fun cap9() {
    var x = 1
    try {
        x = 2
        x = 3
    } finally {
        later3 { use(x) }
    }
}
fun cap10() {
    var x = 1
    val f = fun() { use(x) }
    x = 2
    f()
}
fun cap12() {
    var x = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
    x = 2 // ASSIGNED_VALUE_IS_NEVER_READ
    fun g() { x = 9 }
    x = 3
    g()
    use(x)
}
fun cap13() {
    var x = 1
    x += 2 // ASSIGNED_VALUE_IS_NEVER_READ
    x = 3
    later3 { use(x) }
}
fun cap14() {
    val x = 1
    later3 {
        var y = 1 // VARIABLE_INITIALIZER_IS_REDUNDANT
        y = 2
        use(y)
        use(x)
    }
}
