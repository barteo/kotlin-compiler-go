// The warnings of conversions to the type a value has already and of ==
// of arrays below, as TestWarningsConversions reads them: each line ends
// with a comment naming the warnings it reports, if any. They follow
// kotlinc 2.4.21's RedundantCallOfConversionMethodChecker and
// ArrayEqualityCanBeReplacedWithContentEquals, and the tests of
// extraCheckers/RedundantCallOfConversionMethod and
// ArrayEqualityCanBeReplacedWithEquals.kt of its diagnostics tests.
@file:Suppress("UNUSED_PARAMETER", "UNUSED_VARIABLE", "UNUSED_EXPRESSION")
@file:OptIn(ExperimentalUnsignedTypes::class)

typealias IntAlias = Int

data class Foo(val name: String)

fun String.toString(param: Boolean): String = if (param) "" else this
fun Boolean.toInt() = if (this) 1 else 0

fun conversions(i: UInt, a: IntAlias, s: String?, foo: Foo?, l: Long, c: Char, b: Boolean) {
    val x1 = 1.toInt()
    val x2 = (2 + 3).toInt()
    val x3 = 6U.toUInt()
    val x4 = 1.toLong()
    val x5 = a.toInt() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x6 = Long.MAX_VALUE.toLong() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x7 = 1.1.toDouble() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x8 = i.toUInt() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x9 = "".toString() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x10 = 'a'.toChar() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x11 = 5L.toLong() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x12 = l.toInt()
    val x13 = s.toString()
    val x14: String = foo?.name.toString()
    val x15 = foo?.name?.toString() // REDUNDANT_CALL_OF_CONVERSION_METHOD
    val x16 = "a".toString(true)
    val x17 = b.toInt()
    val x18 = c.code.toChar()
}

fun <T> eq(a: T, b: T): Boolean =
    if (a is IntArray && b is IntArray) a == b else false // ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS

fun arrays(c: Any?, u: UIntArray, v: UIntArray, l: LongArray) {
    val a = arrayOf("a")
    val b = arrayOf(1)
    a == b // ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS
    a != a // ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS
    u == v // ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS
    a == c
    a === a
    l == l // ARRAY_EQUALITY_OPERATOR_CAN_BE_REPLACED_WITH_CONTENT_EQUALS
}
