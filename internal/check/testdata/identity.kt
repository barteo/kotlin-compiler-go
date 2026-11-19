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
