// The warnings kotlinc 2.4.21 reports of the uses of typealiases of deprecated classes below, as
// TestWarningsAliasExpansionDeprecated reads them: each line ends with a comment naming
// the warnings it reports, if any.
@Deprecated("")
class Foo
@Deprecated("")
open class Open
open class WithCtor @Deprecated("") constructor()
class G<T>

typealias Test1 = Foo // DEPRECATION
typealias Test2 = List<Foo> // DEPRECATION
typealias Test3 = List<Test2> // TYPEALIAS_EXPANSION_DEPRECATION
typealias Test4 = G<Test1> // TYPEALIAS_EXPANSION_DEPRECATION
typealias OpenA = Open // DEPRECATION
typealias CtorA = WithCtor
typealias ListA = ArrayList<Foo> // DEPRECATION
typealias Fn = (Foo) -> Unit // DEPRECATION
typealias Nul = Foo? // DEPRECATION

fun use1(b: Test1) = b // TYPEALIAS_EXPANSION_DEPRECATION
fun use2(b: Test2) = b // TYPEALIAS_EXPANSION_DEPRECATION
fun use3(b: Test3) = b // TYPEALIAS_EXPANSION_DEPRECATION
fun use4(b: List<Test1>): Test1? = null // TYPEALIAS_EXPANSION_DEPRECATION
fun use5(b: Fn, c: Nul) {} // TYPEALIAS_EXPANSION_DEPRECATION
val p1: Test1? = null // TYPEALIAS_EXPANSION_DEPRECATION
val p2: Test2? = null // TYPEALIAS_EXPANSION_DEPRECATION
class D1 : OpenA() // TYPEALIAS_EXPANSION_DEPRECATION
val v1 = OpenA() // TYPEALIAS_EXPANSION_DEPRECATION
class D2 : CtorA() // TYPEALIAS_EXPANSION_DEPRECATION
val v2 = CtorA() // TYPEALIAS_EXPANSION_DEPRECATION
val v3 = ListA() // TYPEALIAS_EXPANSION_DEPRECATION
fun <T : Test1> gen(t: T) {} // FINAL_UPPER_BOUND TYPEALIAS_EXPANSION_DEPRECATION
fun body() {
    val x: Test1? = null // TYPEALIAS_EXPANSION_DEPRECATION
    val y = null as Test2? // TYPEALIAS_EXPANSION_DEPRECATION
    val z = x is Test1 // TYPEALIAS_EXPANSION_DEPRECATION
    val r = Test1::class // TYPEALIAS_EXPANSION_DEPRECATION
    val l = listOf<Test1>() // TYPEALIAS_EXPANSION_DEPRECATION
}
@Suppress("TYPEALIAS_EXPANSION_DEPRECATION")
fun sup(b: Test1) = b
fun <T : Foo> gen3(t: T) {} // DEPRECATION FINAL_UPPER_BOUND
fun <T : Test1> gen2(t: T) {} // FINAL_UPPER_BOUND TYPEALIAS_EXPANSION_DEPRECATION
class K<T : Test1> // FINAL_UPPER_BOUND TYPEALIAS_EXPANSION_DEPRECATION
val r3 = Foo::class // DEPRECATION
val r4 = Test2::class // TYPEALIAS_EXPANSION_DEPRECATION
