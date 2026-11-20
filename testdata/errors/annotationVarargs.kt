// The arguments of an annotation's vararg parameter: an array is one
// element in its place, and spread there, and an array factory makes an
// array of its own class. Only a vararg's place takes a spread.
annotation class Strings(vararg val s: String)
annotation class StringArray(val a: Array<String>)

@Strings(*[""])
fun spread() {}

@Strings([""]) // ERROR argument type mismatch
fun literal() {}

@Strings(s = [""])
fun named() {}

@Strings(s = arrayOf())
fun factory() {}

@Strings(s = intArrayOf()) // ERROR argument type mismatch
fun primitiveFactory() {}

@StringArray(["/"])
fun array() {}

@StringArray(*["/"]) // ERROR the spread operator
fun nonVarargSpread() {}
