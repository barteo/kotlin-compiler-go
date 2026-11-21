// The warnings kotlinc 2.4.21 reports of the identity comparisons below,
// as TestWarningsIdentity reads them: each line ends with a comment naming
// the warnings it reports, if any.
@file:Suppress("FINAL_UPPER_BOUND", "PLATFORM_CLASS_MAPPED_TO_KOTLIN", "SENSELESS_COMPARISON")
class Box<T>(val t: T)
fun <T : Int> p1(t: T, i: Int) = t === i // DEPRECATED_IDENTITY_EQUALS
fun <T> p2(t: T, i: Int) = t === i // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun <T : Any> p3(t: T, i: Int) = t === i // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i1(a: Int, b: Int) = a === b // DEPRECATED_IDENTITY_EQUALS
fun i2(a: Int, b: Int) = a !== b // DEPRECATED_IDENTITY_EQUALS
fun i3(a: Int, b: Int?) = a === b // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i4(a: Int?, b: Int) = a === b // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i5(a: Int, x: Any) = a === x // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i6(x: Any?, a: Int) = x !== a // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i7(a: Int, n: Number) = a === n // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i8(a: Int) = a === null
fun i9(a: Int) = a === 1 // DEPRECATED_IDENTITY_EQUALS
fun i10(c: Char) = c === 'c' // DEPRECATED_IDENTITY_EQUALS
fun i11(z: Boolean) = z === true // DEPRECATED_IDENTITY_EQUALS
fun i12(d: Double, f: Double) = d === f // DEPRECATED_IDENTITY_EQUALS
fun i13(s: String, t: String) = s === t
fun i14(x: Any, y: Any) = x === y
fun i15(a: Int, b: Box<Int>) = a === b.t // DEPRECATED_IDENTITY_EQUALS
fun i16(a: Int) = listOf(1)[0] === a // DEPRECATED_IDENTITY_EQUALS
fun i17(x: Any, a: Int) = if (x is Int) x === a else false // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i18(x: Any, y: Any?) = if (x is Int) x === y else false // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i19(a: Int) = a === java.lang.Integer.valueOf(1) // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i20(a: Long, b: Long) = when { a === b -> 1; else -> 2 } // DEPRECATED_IDENTITY_EQUALS
fun i21(a: Int?, b: Int?) = a == b
fun i22(a: Int, b: Int) = a == b
fun i23(a: Any?, b: Any?) = if (a is Int && b is Int) a === b else false // DEPRECATED_IDENTITY_EQUALS
fun <T : Comparable<Int>> i27(t: T, i: Int) = t === i // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i28(a: Int, c: Comparable<Int>) = a === c // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun i29(a: Int, c: Comparable<String>) = a === c // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j1(a: Int, c: Comparable<*>) = a === c // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j2(a: Int, c: java.io.Serializable) = a === c // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun <T : Number> j3(t: T, i: Int) = t === i // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j4(a: Int, c: Comparable<Int>?) = a === c // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j5(a: Int, n: Number?) = a === n // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j6(a: Int, n: Any?) = a === n // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j7(c: Char, n: Comparable<Char>) = c === n // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j8(a: Int, c: Comparable<Int>) = c === a // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j9(a: Any, c: Comparable<Int>) = if (a is Int) a === c else false // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun j10(a: Long, n: Number) = a === n // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun <T : Comparable<Int>> j11(t: T, i: Int) = t === i // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun <T : Any?> j12(t: T, i: Int) = t === i // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun <T : Number?> j13(t: T, i: Int) = t === i // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
@JvmInline value class Vc(val i: Int)
@JvmInline value class Vd(val b: Boolean)
interface Ib
fun fa1(a: Int?, b: Int?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fa2(a: Int?, b: Int) = a === b // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun fa7(a: Int?, b: Any?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fa8(a: Long?, b: Long?) = a !== b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fa9(a: Char?, b: Char?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fa10(a: Boolean?, b: Boolean?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fa11(a: Double?, b: Double?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fa14(a: String?, b: String?) = a === b
fun fs1(x: Any, y: Any) = x is Float && y is Double && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fs2(c: Any?, d: Any?) = c is Vc && d is Vd && c === d // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fs3(a: Any?, b: Any?) = a is Int? && b is String? && a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fs4(c: Any?, d: Any?) = c is Vc? && d is Vd? && c === d // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fs5(x: Any, y: Any) = x is Int && y is Int && x === y // DEPRECATED_IDENTITY_EQUALS
fun fs6(x: Any, y: Any) = x is Int && y is String && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fs7(x: Any, y: Any) = x is Int && y is Ib && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fs8(x: Any?, y: Int?) = x is String && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fs9(x: Any, y: Int) = x is String && x === y // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun fs10(x: Int?, y: Any?) = x != null && y is Long && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Int, K : Boolean> ft1(a: T, b: K) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Int?, K : Boolean?> ft2(a: T, b: K) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Int> ft3(a: T, b: T) = a === b // DEPRECATED_IDENTITY_EQUALS
fun <T : Int?> ft4(a: T, b: Int?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fb1(a: Int?, b: Any) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fb2(a: Int?, b: Number?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fb3(a: Int?, b: Comparable<Int>?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T> fb4(a: Int?, b: T) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Any> fb5(a: Int?, b: T) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fb6(a: Int?, b: Nothing?) = a === b
fun fb7(a: Int?, b: Number) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fb8(a: Any?, b: Int?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fb9(a: java.io.Serializable?, b: Int?) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Int, K : Boolean> fc1(a: T, b: K) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Int?> fc4(a: T, b: T) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Int?, K : Int?> fc5(a: T, b: K) = a === b // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun <T : Int?> fc6(a: T, b: Int) = a === b // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun <T : Int> fc7(a: T, b: Int?) = a === b // IMPLICIT_BOXING_IN_IDENTITY_EQUALS
fun fd1(x: Any, y: Any) = x is Vc && y is Vc && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fd2(x: Any?, y: Int?) = x is Int && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fd3(x: Any?, y: Any?) = x is Int? && y is Int? && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fd4(x: Any?, y: Any?) = x is Int && y is Ib && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fd5(x: Number?, y: Any?) = x is Int && y is String && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fd6(x: Any?, y: Int?) = x is Long && x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
fun fe1(x: Long?, y: Long?) = x === y // FORBIDDEN_IDENTITY_EQUALS_WARNING
