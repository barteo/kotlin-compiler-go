// The warnings kotlinc 2.4.21 reports of the types and templates below, with -Wextra, as
// TestWarningsRedundantTypes reads them: each line ends with a comment naming
// the warnings it reports, if any.
@Target(AnnotationTarget.TYPE)
annotation class TA
typealias U = Unit
interface I {
    fun abs(): Unit // REDUNDANT_RETURN_UNIT_TYPE
    fun dflt(): Unit {} // REDUNDANT_RETURN_UNIT_TYPE
}
abstract class A : I {
    fun foo(): Unit // REDUNDANT_RETURN_UNIT_TYPE
    {
    }
    fun bar(): Unit { // REDUNDANT_RETURN_UNIT_TYPE
        return Unit
    }
    fun baz(): Unit = bar()
    fun ann(): @TA Unit {}
    fun qual(): kotlin.Unit {} // REDUNDANT_RETURN_UNIT_TYPE
    fun alias(): U {} // REDUNDANT_RETURN_UNIT_TYPE
    fun nul(): Unit? { return null }
    abstract fun ab(): Unit // REDUNDANT_RETURN_UNIT_TYPE
    override fun abs(): Unit {} // REDUNDANT_RETURN_UNIT_TYPE
    @Suppress("REDUNDANT_RETURN_UNIT_TYPE")
    fun sup(): Unit {}
    val p: Unit get(): Unit {}
    fun String.ext(): Unit {} // REDUNDANT_RETURN_UNIT_TYPE
    fun <T> gen(): Unit {} // REDUNDANT_RETURN_UNIT_TYPE
    external fun ext2(): Unit // REDUNDANT_RETURN_UNIT_TYPE
}
fun top(): Unit {} // REDUNDANT_RETURN_UNIT_TYPE
fun local() {
    fun inner(): Unit {} // REDUNDANT_RETURN_UNIT_TYPE
    val f = fun(): Unit {} // UNUSED_VARIABLE
    val l: () -> Unit = {} // UNUSED_VARIABLE
}
val x = "Hello"
val n: String? = null
val i = 1
val y1 = "$x" // REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE
val y2 = "${x}" // REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE
val y3 = "$n"
val y4 = "$i"
val y5 = "$x "
val y6 = "${x.trim()}" // REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE
val y7 = """$x""" // REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE
val y8 = "${"lit"}" // REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE
val y9 = "$x$x"
val y10 = "${x + x}" // REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE
fun tmpl(c: CharSequence, a: Any) {
    val z1 = "$c" // UNUSED_VARIABLE
    val z2 = "$a" // UNUSED_VARIABLE
    val z3 = "${a as String}" // REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE UNUSED_VARIABLE
    @Suppress("REDUNDANT_SINGLE_EXPRESSION_STRING_TEMPLATE")
    val z4 = "$x" // UNUSED_VARIABLE
}
var s1: String = ""
    set(param: String) { // REDUNDANT_SETTER_PARAMETER_TYPE
        field = "$param "
    }
class My {
    var y: Int = 1
        set(param: Int) { // REDUNDANT_SETTER_PARAMETER_TYPE
            field = param - 1
        }
    var w: Boolean = true
        set(param) {
            field = !param
        }
    var v: Int = 1
        @Suppress("REDUNDANT_SETTER_PARAMETER_TYPE")
        set(param: Int) {
            field = param - 1
        }
    var q: Int = 1
        set(param: kotlin.Int) {} // REDUNDANT_SETTER_PARAMETER_TYPE
}
var My.e: Int
    get() = 1
    set(value: Int) {} // REDUNDANT_SETTER_PARAMETER_TYPE
