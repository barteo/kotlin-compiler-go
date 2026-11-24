// The warnings kotlinc 2.4.21 reports of the whens below, as
// TestWarningsRedundantElse reads them: each line ends with a comment
// naming the warnings it reports, if any. kc doesn't report kotlinc's
// REDUNDANT_ELSE_IN_WHEN of a when over Any with is Any, nor over a
// value of a type parameter's type.
sealed class X { class A : X(); class B : X() }
sealed interface I
object O1 : I
data class D(val x: Int) : I
enum class E { A, B }
fun f1(x: X) = when (x) { is X.A -> 1; is X.B -> 2; else -> 3 } // REDUNDANT_ELSE_IN_WHEN
fun f2(x: X?) = when (x) { is X.A -> 1; is X.B -> 2; else -> 3 }
fun f3(x: X?) = when (x) { is X.A -> 1; is X.B -> 2; null -> 0; else -> 3 } // REDUNDANT_ELSE_IN_WHEN
fun f4(e: E) = when (e) { E.A -> 1; E.B -> 2; else -> 3 } // REDUNDANT_ELSE_IN_WHEN
fun f5(e: E) { when (e) { E.A -> 1; else -> 3 } } // UNUSED_EXPRESSION
fun f6(b: Boolean) = when (b) { true -> 1; false -> 2; else -> 3 } // REDUNDANT_ELSE_IN_WHEN
fun f7(i: I) = when (i) { O1 -> 1; is D -> 2; else -> 3 } // REDUNDANT_ELSE_IN_WHEN
fun f8(x: X) = when (x) { is X -> 1; else -> 2 } // USELESS_IS_CHECK REDUNDANT_ELSE_IN_WHEN
fun f10(e: E) = when (e) { E.A, E.B -> 1; else -> 2 } // REDUNDANT_ELSE_IN_WHEN
fun f11(e: E) = when (e) { E.A -> 1; E.B -> 2; else if e.ordinal > 0 -> 3; else -> 4 } // REDUNDANT_ELSE_IN_WHEN
fun f12(e: E) = when (e) { E.A -> 1; E.B if e.ordinal > 0 -> 2; else -> 3 }
fun f13(e: E): Int { if (e == E.A) return 0; return when (e) { E.B -> 1; else -> 2 } } // REDUNDANT_ELSE_IN_WHEN
fun f14(x: Any) = if (x is E) when (x) { E.A -> 1; E.B -> 2; else -> 3 } else 0 // REDUNDANT_ELSE_IN_WHEN
fun f15(e: E) = when (val y = e) { E.A -> 1; E.B -> 2; else -> y.ordinal } // REDUNDANT_ELSE_IN_WHEN
fun f17(e: E) = when {
    e == E.A -> 1
    else -> 2
}
fun f18(b: Boolean) = when (b) { true, false -> 1; else -> 2 } // REDUNDANT_ELSE_IN_WHEN
fun f19(e: E) = when (e) { in listOf(E.A) -> 1; E.B -> 2; else -> 3 }
fun f20(b: Boolean) = when (b) { !false -> 1; false -> 2; else -> 3 }
fun f21(e: E) = when (e) {
    E.A -> 1
    E.B -> 2
    else -> { // REDUNDANT_ELSE_IN_WHEN
        3
    }
}
