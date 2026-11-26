// An operator function of integer literals called by name is an Int
// where a Byte is expected, as 2 + 1 is, and inc() and dec() an Int where
// a Long is.
fun takeByte(b: Byte) {}
fun takeLong(l: Long) {}

val l4: Long = (2 + 2 * 3).inc() // ERROR inferred type is Int but Long was expected

fun f() {
    takeByte(2.plus(1)) // ERROR inferred type is Int but Byte was expected
    takeByte(2.inv()) // ERROR inferred type is Int but Byte was expected
    takeLong(1.dec()) // ERROR inferred type is Int but Long was expected
}
