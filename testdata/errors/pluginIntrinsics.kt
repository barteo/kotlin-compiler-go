// kc's intrinsics for the code that compiler plugins generate aren't
// visible to the program.

fun coerce(x: Int): Int = `kc$coerce`<Int>(x) // ERROR unresolved reference: kc$coerce
