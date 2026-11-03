// Annotations on local declarations, statements and expressions: where
// they are written, whether they repeat, and whether their arguments are
// constants.

@Target(AnnotationTarget.FUNCTION) annotation class Fn
@Target(AnnotationTarget.EXPRESSION) @Retention(AnnotationRetention.SOURCE) annotation class Ex
@Target(AnnotationTarget.LOCAL_VARIABLE) annotation class Local
@Target(AnnotationTarget.VALUE_PARAMETER) annotation class Param
annotation class Plain(val i: Int = 0)
annotation class Names(vararg val s: String)

const val CONST = 1
const val TEXT = "text"

fun g(x: Int) = x
fun run(f: () -> Int) = f()

class Init {
    @Plain init {} // ERROR this annotation is not applicable to target 'initializer'
    @Suppress("UNUSED") val p = 1
}

fun locals(arg: Int): Int {
    @Fn val a = 1 // ERROR this annotation is not applicable to target 'local variable'
    @Local @Plain val b = 2
    @Suppress("UNUSED_VARIABLE") val c = 3
    @Local val (d, e) = Pair(1, 2)
    @Ex val f = 4 // ERROR this annotation is not applicable to target 'local variable'
    @Plain @Plain val h = 5 // ERROR this annotation is not repeatable

    @Fn // ERROR this annotation is not applicable to target 'expression'
    while (arg < 0) {}
    @Ex for (i in 1..2) {}
    @Ex do {} while (arg < 0)

    @Plain g(1) // ERROR this annotation is not applicable to target 'expression'
    @Ex g(2)
    @Suppress("UNCHECKED_CAST") g(3)
    val i = g(@Ex 4) + @Ex 5
    val j = g(@Plain 6) // ERROR this annotation is not applicable to target 'expression'
    var k = 0
    @Ex k = @Ex 7
    @Ex ++k
    val l = @Ex if (k > 0) 1 else 2
    val m = @Ex when (k) { 1 -> 1; else -> 2 }

    run @Fn { 1 }
    run(@Plain { 2 })
    run(@Fn fun(): Int { return 3 })
    run(@Fn ::f) // ERROR this annotation is not applicable to target 'expression'
    (@Fn (k)).toString() // ERROR this annotation is not applicable to target 'expression'

    @Fn fun local(@Param x: Int, @Fn y: Int) = x + y // ERROR this annotation is not applicable to target 'value parameter'
    @Plain fun local2() = 1
    @Local fun local3() = 1 // ERROR this annotation is not applicable to target 'local function'

    @Plain class LocalClass
    @Fn class LocalClass2 // ERROR this annotation is not applicable to target 'class'

    try {
    } catch (@Param @Local @Suppress("x") t: Throwable) {
    }
    try {
    } catch (@Fn t: Throwable) { // ERROR this annotation is not applicable to target 'value parameter'
    }

    val local = 8
    @Plain(local) val n = 9 // ERROR annotation argument must be a compile-time constant
    @Plain(CONST + 1) val o = 10
    @Names(TEXT, "$TEXT$CONST", "a" + TEXT) val q = 11
    @Names(TEXT, "$local") val r = 12 // ERROR annotation argument must be a compile-time constant
    return a + b + c + d + e + f + h + i + j + k + l + m + n + o + q + r
}

fun f() = 0
