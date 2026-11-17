// A bound reference on a nullable receiver is an error, except to
// Any?.toString() and the stdlib's Any?.hashCode(), whose receiver may be
// null: x::toString on an Int? or a T is a () -> String.
class C { fun m() {} }

fun <T> f1(x: T): () -> String = x::toString
fun f2(x: Int?): () -> String = x::toString
fun f3(x: Int?): Any = x::toString
fun f4(x: String?): () -> Int = x::hashCode
fun f5(x: Int?): Any = x::equals // ERROR nullable receiver
fun f6(c: C?): Any = c::m // ERROR nullable receiver
fun <S : String?> f7(s: S): Any = s::length // ERROR nullable receiver
fun f8(x: Int?): Any = x::inc // ERROR nullable receiver
fun f9(x: Int): Any = x::toString
fun f10(l: List<*>): Any = l[0]::toString
fun f11(x: Int?) = x::toString
fun g() {
    val r = f11(null)
    val s: String = r()
}
