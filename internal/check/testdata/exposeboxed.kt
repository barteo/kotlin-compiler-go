// The warnings kotlinc 2.4.21 reports of the @JvmExposeBoxed annotations below, as
// TestWarningsUselessExposeBoxed reads them: each line ends with a comment naming
// the warnings it reports, if any.
@file:OptIn(ExperimentalStdlibApi::class)

@JvmInline value class IC(val x: Int) {
    @JvmExposeBoxed fun member() {}
    @JvmExposeBoxed constructor(s: String) : this(1)
}
class Foo @JvmExposeBoxed constructor() { // USELESS_JVM_EXPOSE_BOXED
    @JvmExposeBoxed constructor(i: Long) : this() // USELESS_JVM_EXPOSE_BOXED
    @JvmExposeBoxed constructor(i: IC) : this()
    @JvmExposeBoxed fun m1() {} // USELESS_JVM_EXPOSE_BOXED
    @JvmExposeBoxed fun m2(i: IC) {}
    @JvmExposeBoxed fun m3(): IC = IC(1)
    @JvmExposeBoxed fun IC.m4() {}
    @JvmExposeBoxed fun m5(i: IC?) {}
    @JvmExposeBoxed fun m6(i: List<IC>) {} // USELESS_JVM_EXPOSE_BOXED
    @get:JvmExposeBoxed val p1: Int get() = 1 // USELESS_JVM_EXPOSE_BOXED
    @get:JvmExposeBoxed val p2: IC get() = IC(1)
    val p4: Int @JvmExposeBoxed get() = 1 // USELESS_JVM_EXPOSE_BOXED
    @set:JvmExposeBoxed var p5: Int get() = 1 // USELESS_JVM_EXPOSE_BOXED
        set(v) {}
    @set:JvmExposeBoxed var p6: IC get() = IC(1)
        set(v) {}
}
class Bar @JvmExposeBoxed constructor(val i: IC)
@JvmExposeBoxed // USELESS_JVM_EXPOSE_BOXED
fun Int.foo(i: Int) {}
@JvmExposeBoxed
fun top1(i: IC) {}
@JvmExposeBoxed("named")
fun top2(): IC = IC(1)
@JvmExposeBoxed("named3") // USELESS_JVM_EXPOSE_BOXED
fun top3() {}
@JvmExposeBoxed // USELESS_JVM_EXPOSE_BOXED
fun <T : IC> top4(t: T) {} // FINAL_UPPER_BOUND
@JvmExposeBoxed
fun top5(u: UInt) {}
@JvmExposeBoxed
fun top6(r: Result<Int>) {}
@Suppress("USELESS_JVM_EXPOSE_BOXED") @JvmExposeBoxed
fun top7() {}
@JvmExposeBoxed
class K { fun a() {}; fun b(i: IC) {} }
@JvmExposeBoxed
object Ob { fun a() {} }
typealias TA = IC
@JvmExposeBoxed
fun top8(t: TA) {}
@JvmExposeBoxed // USELESS_JVM_EXPOSE_BOXED
fun top9(f: (IC) -> Unit) {}
