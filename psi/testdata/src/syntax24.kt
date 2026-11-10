// Other Kotlin 2.4 lexer and parser changes.
val leadingZero = 0123
val zeroUnderscore = 0_1
val zeros = 00L
val hex = 0x1F
val bin = 0b101
val dbl = 01.5
val flt = 1f + .5e3F + 1e-2

val s1 = $$"price: $x and $${'$'}{y} and $$x"
val s2 = $$"$$${y}"
val s3 = $$$"$$$$x $$$${x} $$ $"
val s4 = "$$$"
val s5 = $$"""raw $x $$y ${z}"""

fun ops(x: Any, y: List<Any>, a: Int, b: Int) {
    val i1 = x is String in y
    val i2 = x !is Int == true
    val i3 = a shl 2 + b
    val i4 = a as Int * 2
    val i5 = a open b
    val i6 = a data b to a
    val i7 = x is String && x as? String ?: "" == ""
    val i8 = a..b step 2
    val i9 = a in 0..<b
}

fun calls(a: Int) {
    f(a,,a)
    f(,)
    f(a, b = , c)
    listOf(1,,2)
    m[a,,a]
    val l = [1,,2]
}

fun whens(x: Any, y: Boolean) {
    when (@Suppress("x") x) {
        is String && y -> 1
        is Int if y -> 2
        else && y -> 3
        else -> 4
    }
    when (val v = x) { else -> v }
}

class Props {
    val a: Int get() = 1
    val b: Int
        get() { return 2 }
    var c: Int = 0
        get(): Int = field
        set(value) { field = value }
    val d: Int get(x) = 1
    val e: Int get
}

@all:Suppress("x") val annotatedAll = 1
class Ann(@all:Volatile val p: Int, @param:Suppress("x") val q: Int)

val header = 1
val impl = header
fun header(impl: Int) = impl
class Contract { val contract = 1 }

@A(val x: Int) fun annotationVal() {}
