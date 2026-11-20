// The warnings kotlinc 2.4.21 reports of the whens below, as
// TestWarningsDuplicateConditions reads them: each line ends with a
// comment naming the warnings it reports, if any.
enum class Color { RED, GREEN }
object Obj
fun f1(x: Long) = when (x) { 1L -> 1; 1L -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f2(x: Int) = when (x) { -1 -> 1; -1 -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f3(x: Int) = when (x) { 1, 2 -> 1; 1 -> 2; else -> 3 }
fun f4(x: Int) = when (x) { 1 -> 1; 1, 2 -> 2; else -> 3 }
fun f5(x: Int, b: Boolean) = when (x) { 1 if b -> 1; 1 -> 2; 1 -> 3; else -> 4 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f6(x: Int) = when (x) { (1) -> 1; (1) -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f7(x: Any?) = when (x) { is String? -> 1; is String -> 2; is String? -> 3; !is Int -> 4; !is Int -> 5; else -> 6 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN USELESS_IS_CHECK
fun f8(x: Any?) = when (x) { is List<*> -> 1; is List<*> -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f9(x: Char) = when (x) { 'a' -> 1; 'a' -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f10(x: Double) = when (x) { 1.0 -> 1; 1.0 -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f11(x: Boolean) = when (x) { true -> 1; true -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f12(x: String) = when (x) { "a$x" -> 1; "a$x" -> 2; "" -> 3; "" -> 4; else -> 5 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f13(x: Int) = when (x) { Int.MAX_VALUE -> 1; Int.MAX_VALUE -> 2; java.lang.Integer.MIN_VALUE -> 3; java.lang.Integer.MIN_VALUE -> 4; else -> 5 }
fun f14(c: Color) = when (val y = c) { Color.RED -> 1; Color.RED -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f15(x: Int) = when { x == 1 -> 1; x == 1 -> 2; else -> 3 }
fun f16(x: Any) = when (x) { Obj -> 1; Obj -> 2; Color.RED -> 3; Color.RED -> 4; else -> 5 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f17(x: Long) = when (x) { 1L -> 1; 0x1L -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f18(x: Any) = when (x) { 1 -> 1; 1L -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f19(x: Any) = when (x) { 'a' -> 1; "a" -> 2; else -> 3 }
fun f20(x: UInt) = when (x) { 1u -> 1; 1u -> 2; else -> 3 } // DUPLICATE_BRANCH_CONDITION_IN_WHEN
fun f21(x: Any) = when (x) { 1.0f -> 1; 1.0 -> 2; else -> 3 }
