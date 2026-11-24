// The warnings kotlinc 2.4.21 reports of the @OptIn annotations and the subclasses below, as
// TestWarningsOptInArguments reads them: each line ends with a comment naming
// the warnings it reports, if any.
@RequiresOptIn
annotation class M
@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class W
annotation class NotM
class Plain

@OptIn // OPT_IN_WITHOUT_ARGUMENTS
fun a() {}
@OptIn() // OPT_IN_WITHOUT_ARGUMENTS
fun b() {}
@OptIn(M::class)
fun c() {}
@OptIn(NotM::class) // OPT_IN_ARGUMENT_IS_NOT_MARKER
fun d() {}
@OptIn(M::class, NotM::class) // OPT_IN_ARGUMENT_IS_NOT_MARKER
fun e() {}
@OptIn(markerClass = [NotM::class]) // OPT_IN_ARGUMENT_IS_NOT_MARKER
fun f() {}
@OptIn(*[NotM::class]) // OPT_IN_ARGUMENT_IS_NOT_MARKER
fun g() {}
@Suppress("OPT_IN_ARGUMENT_IS_NOT_MARKER")
@OptIn(NotM::class)
fun h() {}
typealias TA = NotM
@OptIn(TA::class) // OPT_IN_ARGUMENT_IS_NOT_MARKER
fun i() {}
@OptIn(SinceKotlin::class) // OPT_IN_ARGUMENT_IS_NOT_MARKER
fun j() {}
@OptIn(ExperimentalStdlibApi::class)
fun k() {}
@OptIn(Deprecated::class) // OPT_IN_ARGUMENT_IS_NOT_MARKER
fun l() {}

@SubclassOptInRequired(W::class)
open class Base
@SubclassOptInRequired(M::class)
interface BaseI
class D1 : Base() // OPT_IN_TO_INHERITANCE
@OptIn(W::class)
class D2 : Base()
@W
class D3 : Base()
@SubclassOptInRequired(W::class)
open class D4 : Base()
class D5 : D4() // OPT_IN_TO_INHERITANCE
val o = object : Base() {} // OPT_IN_TO_INHERITANCE
fun local() {
    class L : Base() // OPT_IN_TO_INHERITANCE
}
@SubclassOptInRequired(W::class, M::class)
open class Two
@OptIn(M::class)
class D6 : Two() // OPT_IN_TO_INHERITANCE
