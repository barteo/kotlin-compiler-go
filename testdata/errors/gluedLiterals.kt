// A literal written against an identifier, a number or a keyword is an
// error, as kotlinc reports it.
infix fun Any?.foo(a: Any) {}

fun test(a: Any) {
    a foo"x" // ERROR literal prefixes and suffixes
    1in 0..2 // ERROR literal prefixes and suffixes
    val b = "s"as Any // ERROR literal prefixes and suffixes
    a foo "x"
    println(1 in 0..2)
    println("$a${1}" + 'c')
}

val f: (vararg x: Int) -> Unit = {} // ERROR modifier on parameter in function type
val g: (x: Int = 0) -> Unit = {} // ERROR default value of parameter in function type
