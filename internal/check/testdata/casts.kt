// The warnings kotlinc 2.4.21 reports of the type checks and casts below,
// as TestWarningsCasts reads them: each line ends with a comment naming
// the warnings it reports, if any.
open class A
open class B
interface I
class F : A()
enum class E { X }
object O
fun <T> id(t: T) = t

fun c0(a: A) {
    a as F
}
fun c1(a: A) {
    a as I
}
fun c2(a: A) {
    a as B // CAST_NEVER_SUCCEEDS
}
fun c3(a: A) {
    a as? B // CAST_NEVER_SUCCEEDS
}
fun c4(i: I) {
    i as B
}
fun c5(i: I) {
    i as F // CAST_NEVER_SUCCEEDS
}
fun c6(f: F) {
    f as I // CAST_NEVER_SUCCEEDS
}
fun c7(f: F) {
    f.hashCode()
}
fun c8(a: A?) {
    a as B? // UNSAFE_CAST_RELYING_ON_NULL
}
fun c9(a: A?) {
    a as B // CAST_NEVER_SUCCEEDS
}
fun c10(a: A?) {
    a as? B? // SAFE_CAST_RELYING_ON_NULL
}
fun c11(x: Int) {
    null as String // CAST_NEVER_SUCCEEDS
}
fun c12(x: Int) {
    null as String?
}
fun c14(x: Int) {
    null as Int // CAST_NEVER_SUCCEEDS
}
fun c15(a: A) {
    a as Nothing // CAST_NEVER_SUCCEEDS
}
fun c16(s: String?) {
    s as Nothing // CAST_NEVER_SUCCEEDS
}
fun c17(s: String?) {
    s as Nothing?
}
fun c18(s: String?) {
    s as? Nothing // CAST_NEVER_SUCCEEDS
}
fun c19(n: Int) {
    n as Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c20(n: Int) {
    n as Long? // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c21(n: Int?) {
    n as Long? // UNSAFE_CAST_RELYING_ON_NULL
}
fun c22(n: Int?) {
    n as Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c23(n: Int?) {
    n as? Long? // SAFE_CAST_RELYING_ON_NULL
}
fun c24(n: Int) {
    n as? Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c25(n: Int) {
    n as Char // CAST_NEVER_SUCCEEDS
}
fun c26(c: Char) {
    c as Int // CAST_NEVER_SUCCEEDS
}
fun c27(u: UInt) {
    u as Int // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c28(n: Int) {
    n as UInt // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c29(x: Int) {
    1 as String // CAST_NEVER_SUCCEEDS
}
fun c30(x: Int) {
    1 as Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c31(x: Int) {
    1 as Byte // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c32(x: Int) {
    1 as Int?
}
fun c33(x: Int) {
    1 as Any
}
fun c34(x: Int) {
    "s" as Int // CAST_NEVER_SUCCEEDS
}
fun c35(x: Int) {
    (1 + 2) as Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun c36(e: E) {
    e as A // CAST_NEVER_SUCCEEDS
}
fun c37(e: E) {
    e as I // CAST_NEVER_SUCCEEDS
}
fun c38(o: O) {
    o as I // CAST_NEVER_SUCCEEDS
}
fun c39(o: O) {
    o as A // CAST_NEVER_SUCCEEDS
}
fun c40(s: String?) {
    s is String
}
fun c41(s: String?) {
    s is String? // USELESS_IS_CHECK
}
fun c42(s: String?) {
    s !is Any? // USELESS_IS_CHECK
}
fun c43(s: String) {
    s is Any // USELESS_IS_CHECK
}
fun c44(s: String) {
    s is CharSequence // USELESS_IS_CHECK
}
fun c45(x: List<String>) {
    x is List<*> // USELESS_IS_CHECK
}
fun c46(x: List<String>) {
    x is Collection<*> // USELESS_IS_CHECK
}
fun c47(x: List<String>) {
    x is MutableList<*>
}
fun c48(x: Any) {
    if (x is String) x is CharSequence // USELESS_IS_CHECK
}
fun c49(x: Any) {
    if (x is String) x is Int // IMPOSSIBLE_IS_CHECK_WARNING
}
fun c50(x: Any) {
    if (x is String) x as Int // CAST_NEVER_SUCCEEDS
}
fun c51(x: Any?) {
    if (x is String?) x is Int? // IMPOSSIBLE_IS_CHECK_RELYING_ON_NULL_WARNING
}
fun c52(x: Any?) {
    if (x is String?) x as Int? // UNSAFE_CAST_RELYING_ON_NULL
}
fun c53(x: Any) {
    if (x is I) x as B
}
fun c54(x: Any) {
    if (x is I) x is B
}
fun c55(x: A) {
    if (x is I) x as B // CAST_NEVER_SUCCEEDS
}
fun c56(x: A) {
    if (x is I) x is A // USELESS_IS_CHECK
}
fun c57(x: A) {
    if (x is I) x is I // USELESS_IS_CHECK
}
fun c58(x: Any) {
    if (x is A) x as I
}
fun c59(x: Any) {
    if (x is F) x as I // CAST_NEVER_SUCCEEDS
}
fun c60(x: Any) {
    if (x !is String) return; x is CharSequence // USELESS_IS_CHECK
}
fun <T> c61(x: T) {
    x is Any
}
fun <T> c62(x: T) {
    x is Any? // USELESS_IS_CHECK
}
fun <T> c63(x: T) {
    x is T // USELESS_IS_CHECK
}
fun <T> c64(x: T) {
    x as String
}
fun <R : A> c65(x: R) {
    x is A // USELESS_IS_CHECK
}
fun <R : A> c66(x: R) {
    x as B // CAST_NEVER_SUCCEEDS
}
fun <R : A> c67(x: R) {
    x as String // CAST_NEVER_SUCCEEDS
}
fun <R : A> c68(x: R?) {
    x as String // CAST_NEVER_SUCCEEDS
}
fun c69(x: java.util.ArrayList<String>) {
    x is List<*> // USELESS_IS_CHECK
}
fun c72(x: Number) {
    x as Int
}
fun c73(x: Number) {
    x is Int
}
fun c74(x: Int) {
    x is Number // USELESS_IS_CHECK
}
fun c75(x: Int) {
    x as Number
}
fun c76(x: Int) {
    x as Comparable<Int>
}
fun c79(x: IntArray) {
    x as Array<Int> // CAST_NEVER_SUCCEEDS
}
fun c80(x: Array<String>) {
    x is Array<*> // USELESS_IS_CHECK
}
fun c81(x: () -> Unit) {
    x as String // CAST_NEVER_SUCCEEDS
}
fun c82(x: () -> Unit) {
    x as Function0<Unit> // USELESS_CAST
}
fun c83(x: () -> Unit) {
    x is Function<*> // USELESS_IS_CHECK
}
fun c84(x: String) {
    x as String // USELESS_CAST
}
fun c85(x: Any) {
    x as String
}
fun c86(x: Unit) {
    x as String // CAST_NEVER_SUCCEEDS
}
fun c87(x: Nothing?) {
    x as String // CAST_NEVER_SUCCEEDS
}
interface J
sealed class S { class S1 : S(); class S2 : S() }

fun w0(x: String) = when (x) {
    is String -> 1 // USELESS_IS_CHECK
}
fun w2(x: String, b: Boolean) = when (x) {
    "a" -> 0
    is CharSequence -> 1
}
fun w3(x: String) = when (x) {
    !is CharSequence -> 1 // USELESS_IS_CHECK
    else -> 2
}
fun w4(x: S) = when (x) {
    is S.S1 -> 1
    is S.S2 -> 2
}
fun w5(x: S) = when (x) {
    is S.S1 -> 1
    is S -> 2
}
fun w6(x: String) { when (x) {
    "a" -> 0 // UNUSED_EXPRESSION
    !is CharSequence -> 1 // USELESS_IS_CHECK UNUSED_EXPRESSION
} }
fun w7(x: Int) = when {
    x is Int -> 1 // USELESS_IS_CHECK
    else -> 2
}
fun w8(x: A) = when (x) {
    is I -> 1
    else -> 2
}
fun w10(x: Any) {
    if (x is I && x is J) x as B
}
fun w11(x: Any) {
    if (x is I && x is A) x as B
}
fun w12(x: Any) {
    if (x is I && x is J) x is A
}
fun w13(x: Any) {
    if (x is A && x is I) x is B
}
fun w14(x: Any) {
    if (x is String) when (x) { is Int -> 1; else -> 2 } // IMPOSSIBLE_IS_CHECK_WARNING UNUSED_EXPRESSION
}
fun w15(x: Any) {
    if (x is A) x is F
}
fun w16(x: Any) {
    if (x is F) x is A // USELESS_IS_CHECK
}
fun w17(x: Any?) {
    if (x is A?) x is B? // IMPOSSIBLE_IS_CHECK_RELYING_ON_NULL_WARNING
}
fun w18(x: Any?) {
    if (x is A?) x as B? // UNSAFE_CAST_RELYING_ON_NULL
}
fun w19(x: Any?) {
    if (x is A?) x as? B? // SAFE_CAST_RELYING_ON_NULL
}
fun w20(x: Any?) {
    if (x is A?) x as? B // CAST_NEVER_SUCCEEDS
}
fun w21(x: Any) {
    if (x is Int) x as Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun w22(x: Any) {
    if (x is Int) x as Long? // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun w23(x: Any) {
    if (x is Int) x !is Long // IMPOSSIBLE_IS_CHECK_WARNING
}
fun w24(x: Number) {
    if (x is Int) x as Long // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
}
fun w25(x: Any) {
    if (x is Int?) x as String // CAST_NEVER_SUCCEEDS
}
fun w26(x: Any?) {
    if (x is Int?) x as String // CAST_NEVER_SUCCEEDS
}
fun w27(x: Any?) {
    if (x != null) x is Any // USELESS_IS_CHECK
}
fun w28(x: String?) {
    if (x != null) x is String // USELESS_IS_CHECK
}
fun w29(x: Any?) {
    if (x is String) x is String? // USELESS_IS_CHECK
}
fun w30(x: Any) {
    if (x is B) when (x) { is A -> 1; else -> 2 } // IMPOSSIBLE_IS_CHECK_WARNING UNUSED_EXPRESSION
}
fun w31(x: Any) {
    if (x is A) { if (x is I) x is B }
}
fun w32(x: Any) {
    x as String
    x as Int // CAST_NEVER_SUCCEEDS
}
fun w33(x: Any) {
    x as String
    x is CharSequence // USELESS_IS_CHECK
}
fun w34(x: Int?) {
    val a1 = x as? Int // USELESS_CAST
    val a2 = x as? Int? // USELESS_CAST
    val a3 = x as? Number
    val a6: Number? = x as? Int // USELESS_CAST
    val a7: Number? = 1 as? Number
    if (x is Int) {
        val b = x as? Int // USELESS_CAST
    }
}
fun w35(s: String) {
    s as? String // USELESS_CAST
    s as? String?
}
fun w36() {
    1.0 as Double // USELESS_CAST
    1f as? Float // USELESS_CAST
    1L as Long // USELESS_CAST
    1.0 as Double?
}
fun w37(x: Int, l: Long) {
    1 as Int // INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL
    1 as? Int // INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL
    -1 as Int // INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL
    (1 + 2 * 3) as Int // INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL
    (1 shl 2) as Int // INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL
    1.plus(2) as Int // INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL
    1u as UInt // INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL
    1UL as ULong // USELESS_CAST
    (1 + x) as Int // USELESS_CAST
    (1 + 2L) as Long // USELESS_CAST
    1.inc() as Int // USELESS_CAST
    1.toInt() as Int // USELESS_CAST
    1 as Number
    @Suppress("INTEGER_LITERAL_CAST_INSTEAD_OF_TO_CALL") val a = 1 as Int
}
// Casts of generic calls, useless unless the function takes its type
// argument from the cast, and of type parameters' values.
fun uUse(a: Any?) {}
fun <T> uFoo(a: T) = a
fun <T> uGen(): T = null as T // UNCHECKED_CAST
fun <T> uLst(vararg a: T): List<T> = a.toList()
class UBox<T>(val v: T)
class UG<T>(val v: T) {
    fun self() = this as UG<T> // USELESS_CAST
    fun get(): T = v as T // USELESS_CAST
    fun <R> map(f: (T) -> R): R = f(v) as R // USELESS_CAST
}
fun <T> T.uc1(): T = uFoo(this) as T // USELESS_CAST
fun <T> uc2(v: T): T = v as T // USELESS_CAST
fun <T> uc3(v: T): T = uFoo(v) as T // USELESS_CAST
fun uc4() { uUse(uFoo(1) as Int) } // USELESS_CAST
fun uc5() { uUse(uGen() as String) }
fun uc6() { uUse(uFoo("a") as String) } // USELESS_CAST
fun uc7() { uUse(uLst(1, 2) as List<Int>) } // USELESS_CAST
fun uc8() { uUse(UBox(1) as UBox<Int>) } // USELESS_CAST
fun uc9() { uUse(listOf<Int>() as List<Int>) } // USELESS_CAST
fun uc10() { uUse(emptyList<String>() as List<String>) } // USELESS_CAST
fun <T : Any> uc12(v: T?): T = v!! as T // USELESS_CAST
fun <T> uc13(v: Any): T = v as T // UNCHECKED_CAST
fun uc14(a: Any) { if (a is String) uUse(uFoo(a) as String) } // USELESS_CAST
fun <T> uc15(v: T): T? = v as T?
fun <T> uc16(v: T?) { uUse(v as T?) } // USELESS_CAST
fun uc17() { uUse(mutableListOf(1) as MutableList<Int>) } // USELESS_CAST
fun uc18() { uUse(uFoo(null) as Nothing?) } // USELESS_CAST
fun uc19() { uUse(uFoo(1 as Number) as Number) } // USELESS_CAST
fun uc20() { val x: Long = uFoo(1) as Long; uUse(x) } // NUMERIC_CAST_NEVER_SUCCEEDS_BUT_CAN_BE_REPLACED_WITH_TO_CALL
fun <T> ud2(v: T) { if (v != null) uUse(v as T) }
fun <T> ud3(v: T) { if (v is String) uUse(v as T) }
inline fun <reified T> ud4(v: T) { uUse(v as T) } // USELESS_CAST
fun <T> ud5(v: T?) { uUse(v as? T) } // USELESS_CAST
fun <T : Any> ud6(v: T) { uUse(v as? T) } // USELESS_CAST
fun ud7(l: List<*>) { uUse(l as List<*>) } // USELESS_CAST
fun ud8(l: List<Any?>) { uUse(l as List<*>) } // USELESS_CAST
fun <T> ud9(l: List<T>) { uUse(l[0] as T) } // USELESS_CAST
fun <T> ud10(l: List<T>) { uUse(l.first() as T) } // USELESS_CAST
fun ud11(a: Any) { if (a is String && a is CharSequence) uUse(a as String) } // USELESS_IS_CHECK USELESS_CAST
fun <T : CharSequence> ud12(v: T) { uUse(v as CharSequence) }
fun <T> ud13(v: T & Any) { uUse(v as (T & Any)) } // USELESS_CAST
fun ud14() { uUse(uGen<String>() as String) }
fun ud15() { uUse(uFoo<String>("a") as String) } // USELESS_CAST
fun <T> ud16(v: T) { val f = { v }; uUse(f() as T) } // USELESS_CAST
fun ud17(s: String?) { uUse(s!! as String) } // USELESS_CAST
fun <T> ud18(v: T) { uUse(listOf(v) as List<T>) } // USELESS_CAST
fun ud19(x: Int?) { uUse(x as? Int) } // USELESS_CAST
fun <T> ud20(a: Array<T>) { uUse(a[0] as T) } // USELESS_CAST
fun <E : Enum<E>> ud21(e: E) { uUse(e as E) } // USELESS_CAST
fun ud22(m: Map<String, Int>) { uUse(m["a"] as Int?) } // USELESS_CAST
fun <T> ud23(v: T) { var w = v; w = uFoo(w) as T; uUse(w) } // USELESS_CAST
