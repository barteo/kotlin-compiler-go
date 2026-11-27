// The warnings kotlinc 2.4.21 reports of the suppressions below, as
// TestWarningsErrorSuppression reads them: each line ends with a comment
// naming the warnings it reports, if any.
@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
fun <T> f1(a: T, b: @kotlin.internal.NoInfer T): T = a
@Suppress("INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
fun <T> f2(a: T, b: @kotlin.internal.NoInfer T, c: @kotlin.internal.Exact T): T = a
@Suppress("INVISIBLE_MEMBER")
fun f3() {}
@Suppress("INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
fun f4() {}
@Suppress("INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
class C {
    @Suppress("INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
    fun <T> m(a: T, b: @kotlin.internal.NoInfer T): T = a
    fun <T> n(a: T, b: @kotlin.internal.NoInfer T): T = a
}
@Suppress("invisible_reference")
fun <T> f5(a: T, b: @kotlin.internal.NoInfer T): T = a
@Suppress("INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
@kotlin.internal.LowPriorityInOverloadResolution
fun f6() {}
fun use() {
    @Suppress("INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
    fun <T> local(a: T, b: @kotlin.internal.NoInfer T): T = a
    local(1, 2)
}
@Suppress("NOTHING_TO_INLINE")
inline fun g1() {}
@Suppress("CONFLICTING_OVERLOADS", "UNUSED_PARAMETER") // ERROR_SUPPRESSION
fun g2(x: Int) {}
@Suppress("TYPE_MISMATCH", "TYPE_MISMATCH") // ERROR_SUPPRESSION
fun g3(): Int = 1
@Suppress("ACCIDENTAL_OVERRIDE")
fun g4() {}
@Suppress("WRONG_ANNOTATION_TARGET") // ERROR_SUPPRESSION
fun g5() {}
@Suppress("warnings")
fun g6() {}
fun g7(@Suppress("UNRESOLVED_REFERENCE") p: Int) = p // ERROR_SUPPRESSION
class D @Suppress("INVISIBLE_REFERENCE") // ERROR_SUPPRESSION
@kotlin.internal.LowPriorityInOverloadResolution
constructor(val x: Int)
@Target(AnnotationTarget.TYPE)
annotation class TA
fun h1(): @Suppress("REPEATED_ANNOTATION") @TA @TA Int = 42 // ERROR_SUPPRESSION
typealias H2<T> = @Suppress("TYPEALIAS_SHOULD_EXPAND_TO_CLASS") T // ERROR_SUPPRESSION
interface H3
interface H4 : @Suppress("SUPERTYPE_NOT_INITIALIZED") H3 // ERROR_SUPPRESSION
fun h5(x: @Suppress("REDUNDANT_NULLABLE") String??) = x
fun h6(x: List<@Suppress("TYPE_MISMATCH") String>) = x // ERROR_SUPPRESSION
fun h7() {
    val x: @Suppress("UNRESOLVED_REFERENCE") Int = 1 // ERROR_SUPPRESSION
    x.toString()
}
