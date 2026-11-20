// REDUNDANT_NULLABLE: a second ? on a type, or ? on a typealias that is
// nullable already.
typealias A = Int?
typealias B = A
typealias F = (() -> Unit)?
typealias I = Int
class G<T>

fun f1(a: A?) {} // REDUNDANT_NULLABLE
fun f2(b: B?) {} // REDUNDANT_NULLABLE
fun f3(f: F?) {} // REDUNDANT_NULLABLE
fun f4(i: I?) {}
fun f5(p: (Int?)?) {}
fun f6(q: ((Int)?)?) {}
fun f7(r: Int??) {} // REDUNDANT_NULLABLE
fun f8(s: G<Int??>) {} // REDUNDANT_NULLABLE
fun f9(t: (() -> Unit)??) {} // REDUNDANT_NULLABLE
fun f10(u: G<A?>) {} // REDUNDANT_NULLABLE
fun f11(v: Int???) {} // REDUNDANT_NULLABLE
fun f12(w: A) {}
fun f13(x: I??) {}
fun <T> f14(t: T??) {} // REDUNDANT_NULLABLE
fun <T> f15(t: T?) {}
fun f16(): A? = null // REDUNDANT_NULLABLE
fun String??.f17() {} // REDUNDANT_NULLABLE
fun String?.f18() {}
fun f19(a: Any) {
    val x = a is Int?? // REDUNDANT_NULLABLE
    val y: Array<out String??> = arrayOf() // REDUNDANT_NULLABLE
    @Suppress("REDUNDANT_NULLABLE") val z: Int?? = null
}
