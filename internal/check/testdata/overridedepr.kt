// The warnings of overrides of deprecated members below, as
// TestWarningsOverrideDeprecation reads them: each line ends with a
// comment naming the warnings it reports, if any. They follow kotlinc
// 2.4.21's FirOverrideChecker and the tests deprecatedInheritance.kt,
// deprecatedPropertyInheritance.kt and
// duplicatedOverrideDeprecationOnProperty.kt of its diagnostics tests.
@file:Suppress("UNUSED_PARAMETER")

interface W {
    @Deprecated("w")
    fun f() {}
}

interface E {
    @Deprecated("e", level = DeprecationLevel.ERROR)
    fun f() {}
}

interface H {
    @Deprecated("h", level = DeprecationLevel.HIDDEN)
    fun f() {}
}

interface N {
    fun f() {}
}

class WE : W, E {
    override fun f() {} // OVERRIDE_DEPRECATION
}

class WH : W, H {
    override fun f() {} // OVERRIDE_DEPRECATION
}

class NW : N, W {
    override fun f() {}
}

interface E2 : E
interface W2 : W

interface EW2 : E2, W2 {
    override fun f() {} // OVERRIDE_DEPRECATION
}

interface HEW2 : EW2, H {
    override fun f() {}
}

interface Explicit : HEW2 {
    @Deprecated("x", level = DeprecationLevel.ERROR)
    override fun f() {}
}

interface Props {
    @Deprecated("a")
    val a: String

    @Deprecated("b")
    var b: String

    val c: String
}

open class Accessors {
    open var g: Int = 3
        @Deprecated("") get

    open var s: Int = 3
        @Deprecated("") set
}

class P(override val c: String) : Props, Accessors() {
    override val a: String = "" // OVERRIDE_DEPRECATION

    override var b: String = "" // OVERRIDE_DEPRECATION
        get() = field

    override var g: Int = 1 // OVERRIDE_DEPRECATION

    override var s: Int // OVERRIDE_DEPRECATION
        get() = 3
        set(value) {}
}

class Q(@Deprecated("") override val a: String) : Props {
    @Deprecated("")
    override var b: String = ""

    override val c: String
        @Deprecated("") get() = ""
}

class R : Comparable<R> {
    override fun compareTo(other: R): Int = 0
}

// Number's toChar() is deprecated, an error since Kotlin 2.3.
open class NumberChar(val value: Int) : Number() {
    override fun toChar(): Char = '+' // OVERRIDE_DEPRECATION
    override fun toInt(): Int = value
    override fun toByte(): Byte = toInt().toByte()
    override fun toDouble(): Double = toInt().toDouble()
    override fun toFloat(): Float = toInt().toFloat()
    override fun toLong(): Long = toInt().toLong()
    override fun toShort(): Short = toInt().toShort()
}
class NumberChar2(value: Int) : NumberChar(value) {
    override fun toChar(): Char = 'x'
}
fun numberChar(n: NumberChar) = n.toChar()
