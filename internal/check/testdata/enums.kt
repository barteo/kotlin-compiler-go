// The warnings kotlinc 2.4.21 reports of the enum comparisons below, as
// TestWarningsEnumComparison reads them: each line ends with a comment
// naming the warnings it reports, if any. kc compiles the comparisons of
// two nullable enums, which kotlinc only warns of.
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
