// The declarations that testdata/inferredvis/b.kt infers the types of.
package a

private interface P
object O1 : P
object O2 : P
fun makeA(): Any = object {}
