// The warnings kotlinc 2.4.21 reports of the vararg parameters of unsigned types below, as
// TestWarningsUnsignedVararg reads them: each line ends with a comment naming
// the warnings it reports, if any.
fun ubyte(vararg a: UByte) {} // OPT_IN_USAGE
fun ushort(vararg a: UShort) {} // OPT_IN_USAGE
fun uint(vararg a: UInt) {} // OPT_IN_USAGE
fun ulong(vararg a: ULong) {} // OPT_IN_USAGE
class ValueParam(vararg val a: ULong) // OPT_IN_USAGE
class Plain(vararg a: UInt) // OPT_IN_USAGE
annotation class Ann(vararg val a: UInt) // OPT_IN_USAGE
fun plain(a: UInt, vararg b: Int) {}
@OptIn(ExperimentalUnsignedTypes::class)
fun opted(vararg a: UInt) {}
@ExperimentalUnsignedTypes
fun marked(vararg a: UInt) {}
typealias U = UInt
fun alias(vararg a: U) {} // OPT_IN_USAGE
fun nullable(vararg a: UInt?) {}
class Sec { constructor(vararg a: UShort) } // OPT_IN_USAGE
fun local() {
    fun inner(vararg a: UInt) {} // OPT_IN_USAGE
    val f = fun(vararg a: UInt) {} // USELESS_VARARG_ON_PARAMETER
}
fun arr(a: UIntArray) {} // OPT_IN_USAGE

// Their calls: the parameter is of the array class.
@OptIn(ExperimentalUnsignedTypes::class)
fun calls() {
    uint(1u)
    Plain(1u)
}
fun calls2() {
    uint() // OPT_IN_USAGE
    uint(1u, 2u) // OPT_IN_USAGE
    plain(1u)
    Plain(1u) // OPT_IN_USAGE
    Sec(1u) // OPT_IN_USAGE
    opted(1u) // OPT_IN_USAGE
    ValueParam(1uL) // OPT_IN_USAGE
}
