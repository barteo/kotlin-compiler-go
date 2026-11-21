// The UNNECESSARY_LATEINIT warnings kotlinc 2.4.21 reports of the
// lateinit properties below, as TestWarningsLateinit reads them: each
// line ends with a comment naming the warnings it reports, if any.
class Test {
    lateinit var someRunnable: Runnable // UNNECESSARY_LATEINIT
    init {
        someRunnable = Runnable { someRunnable.run() }
    }
}
class Foo {
    lateinit var bar: String // UNNECESSARY_LATEINIT
    init {
        bar = ""
    }
}
class Foo2 {
    lateinit var bar: String
    init {
        println(bar)
        bar = ""
    }
}
class Foo3 {
    lateinit var bar: String // UNNECESSARY_LATEINIT
    init {
        if (System.currentTimeMillis() > 0) bar = "" else bar = "x"
    }
}
class Foo4(x: Int) {
    lateinit var bar: String // UNNECESSARY_LATEINIT
    init { foo() ; bar = "" }
    fun foo() { println(bar) }
}
class Foo5 {
    lateinit var bar: String // UNNECESSARY_LATEINIT
    init { bar = ""; bar = "" }
}
class Foo6 {
    lateinit var bar: String // UNNECESSARY_LATEINIT
    init { this.bar = "" }
}
class Foo7 {
    @Transient lateinit var bar: String
    init { bar = "" }
}
class Foo8 {
    lateinit var bar: String
    init { run { bar = "" } }
}
class Foo9 {
    lateinit var bar: String // UNNECESSARY_LATEINIT
        private set
    init { bar = "" }
}
object O {
    lateinit var bar: String // UNNECESSARY_LATEINIT
    init { bar = "" }
}
lateinit var top: String
fun c(): Boolean = true
class A1 {
    lateinit var x: String
    init { if (c()) x = "" }
}
class A2 {
    lateinit var x: String
    init { try { x = "" } catch (e: Exception) { } }
}
class A3 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { try { x = "" } finally { } }
}
class A4 {
    lateinit var x: String
    init { while (c()) { x = "" } }
}
class A5 {
    lateinit var x: String
    init { println(this.x); x = "" }
}
class A6 {
    lateinit var x: String
    init { x = x + "" }
}
class A7 {
    lateinit var x: String
    val y = x.length
    init { x = "" }
}
class A8 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { x = ""; println(x) }
}
class A9 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { do { x = "" } while (c()) }
}
class A10 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { when { c() -> x = "a"; else -> x = "b" } }
}
class A11 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { if (!c()) throw Exception(); x = "" }
}
class A13 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { x = ""; ::x.isInitialized }
}
class A14(val p: String) {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { x = p }
    constructor() : this("")
}
class A15 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    val y: String
    init { y = ""; x = y }
}
open class B { open fun f() {} }
class A16 : B() {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { f(); x = "" }
}
class A17 {
    private lateinit var x: String // UNNECESSARY_LATEINIT
    init { x = "" }
}
class A18 {
    lateinit var x: String // UNNECESSARY_LATEINIT
    init { x = "" }
    fun reset() { x = "q" }
}
class A19 {
    lateinit var x: String
    init { listOf(1).forEach { x = "" } }
}
