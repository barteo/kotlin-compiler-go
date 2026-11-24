// The warnings kotlinc 2.4.21 reports of the enum comparisons below, as
// TestWarningsEnumComparison reads them: each line ends with a comment
// naming the warnings it reports, if any. kc compiles the comparisons
// below, which kotlinc only warns of: of two nullable types, a type
// parameter's value or a smart-cast one.
enum class E1 { A, B }
enum class E2 { A { override fun toString() = "a" }, B }
fun g1(a: E1?, b: E2?) = a == b // INCOMPATIBLE_ENUM_COMPARISON
fun g2(a: E1?, b: E2?): Boolean { if (a == null) return false; return a == b } // INCOMPATIBLE_ENUM_COMPARISON
fun g4(a: E1?, b: E2?) = when (a) { b -> 1; else -> 2 } // INCOMPATIBLE_ENUM_COMPARISON
fun g7(a: E1?) = a == (null as E2?) // INCOMPATIBLE_ENUM_COMPARISON
var p: E1? = null
fun g8(b: E2?) = p == b // INCOMPATIBLE_ENUM_COMPARISON
fun g9(a: E1?, b: E2?) = a?.equals(b)
fun g10(a: E1?, b: E2?): Boolean { if (a == null || b == null) return false; return a == b } // INCOMPATIBLE_ENUM_COMPARISON
fun g11(a: E1?, b: E2?): Boolean { if (b == null) return false; return a == b } // INCOMPATIBLE_ENUM_COMPARISON
fun g12(a: E1?, b: E2?) = if (a != null) a == b else false // INCOMPATIBLE_ENUM_COMPARISON
fun h1(a: E1?, b: E2?) = a == b // INCOMPATIBLE_ENUM_COMPARISON
fun h2(a: E1?, b: E2?) = a != b // INCOMPATIBLE_ENUM_COMPARISON
fun h3(a: E1?, b: E2?) = a === b
fun h4(a: E1?) = a == null
fun h5(a: E1?, b: E2?) = b == a // INCOMPATIBLE_ENUM_COMPARISON
fun h10(a: E1?, b: Enum<E2>?) = a == b
interface J1
interface J2
open class OpenC
abstract class AbsC
enum class F1 { A, B }
enum class F2 { A, B }
enum class F3 : J1 { X }
enum class F4 : J1, J2 { X }
interface Inv5<T>
enum class F5 : Inv5<Int> { A }
fun <T : J1> kp1(t: T, e: F1) = t == e // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1> kp2(t: T, e: F3) = t == e
fun <T : J1> kp3(t: T, e: F1?) = e == t // INCOMPATIBLE_ENUM_COMPARISON
fun <T> kp4(t: T, e: F1) = t == e
fun <T : Any> kp5(t: T, e: F1) = t == e
fun <T> kp6(t: T, e: F1) where T : J1, T : J2 = t == F3.X // INCOMPATIBLE_ENUM_COMPARISON
fun <T> kp7(t: T, e: F1) where T : J1, T : J2 = t == F4.X
fun <T : OpenC> kp8(t: T) = t == F1.A // INCOMPATIBLE_ENUM_COMPARISON
fun <T : AbsC> kp9(t: T) = F1.A == t // INCOMPATIBLE_ENUM_COMPARISON
fun <T : F1, K : F2> kp10(t: T, k: K) = t == k // INCOMPATIBLE_ENUM_COMPARISON
fun <T : F1, K : F1> kp11(t: T, k: K) = t == k
fun <T : F1> kp12(t: T) = t == F2.A // INCOMPATIBLE_ENUM_COMPARISON
fun <T : F1> kp13(t: T) = t == F1.A
fun <T : J1> ki1(t: T, e: F1) = t === e // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1> ki2(t: T) = F1.A === t // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1> ki3(t: T) = F1.A !== t // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1> ki4(t: T) = t != F1.A // INCOMPATIBLE_ENUM_COMPARISON
fun kn1(a: F1?, b: F2?) = a === b
fun kn2(a: F1?, b: J1?) = a == b // INCOMPATIBLE_ENUM_COMPARISON
fun kn3(a: F3?, b: J1?) = a == b
fun kn4(a: F1?, b: OpenC?) = a == b // INCOMPATIBLE_ENUM_COMPARISON
fun kn5(a: F1?, b: Any?) = a == b
fun kn6(a: F1?, b: Enum<F2>?) = a == b
fun kn7(a: Enum<F1>?, b: Enum<F2>?) = a == b
fun kn8(a: F5?, b: Inv5<String>?) = a == b
fun kn9(a: F1?, b: J1?) = a === b
fun ks1(a: Any?, b: Any?) = a is F1 && b is F2 && a == b // INCOMPATIBLE_ENUM_COMPARISON
fun ks2(a: Any?, b: Any?) = a is F1 && b is J1 && a == b // INCOMPATIBLE_ENUM_COMPARISON
fun ks3(a: Any?, b: Any?) = a is F3 && b is J1 && a == b
fun ks4(a: Any?, b: Any?) = a is F1 && b is F2 && a === b // INCOMPATIBLE_ENUM_COMPARISON
fun ks5(a: Any, b: Any) = a is F1 && b is F2 && a == b // INCOMPATIBLE_ENUM_COMPARISON
fun ks6(a: Any?, b: Any?) = a is F1? && b is F2? && a == b // INCOMPATIBLE_ENUM_COMPARISON
fun ks7(a: F1?, b: Any?) = b is F2 && a == b // INCOMPATIBLE_ENUM_COMPARISON
fun ks8(a: J1, b: Any) = b is F1 && a == b // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1> kw1(t: T) = when (t) { F1.A -> 1; else -> 2 } // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1> kw2(t: T) = when (F1.A) { t -> 1; else -> 2 } // INCOMPATIBLE_ENUM_COMPARISON
fun kw3(e: F1?, f: F2?) = when (e) { f -> 1; else -> 2 } // INCOMPATIBLE_ENUM_COMPARISON
fun kw4(e: F1?, f: J1?) = when (e) { f -> 1; else -> 2 } // INCOMPATIBLE_ENUM_COMPARISON
fun kw5(e: Any?) = if (e is F1) when (e) { F2.A -> 1; else -> 2 } else 3 // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1> kw6(e: F1?, t: T) = when (e) { t -> 1; else -> 2 } // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1?> kq1(t: T, e: F1?) = t == e // INCOMPATIBLE_ENUM_COMPARISON
fun <T : J1, K : T> kq2(k: K, e: F1) = k == e // INCOMPATIBLE_ENUM_COMPARISON
fun <T> kq3(t: T, e: F1?) where T : J1, T : OpenC = t == e // INCOMPATIBLE_ENUM_COMPARISON
fun kbar(x: Any) = if (x is F1 && x is F2) when (x) { F1.A -> 1; F2.A -> 2; else -> 3 } else 0 // IMPOSSIBLE_IS_CHECK_WARNING
