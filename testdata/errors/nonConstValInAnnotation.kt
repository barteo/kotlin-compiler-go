// A val that could be a const val, read where a constant is needed, is
// kotlinc's NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION; a var, a val with a
// getter or a non-constant initializer, and a local val are not.

annotation class Ann(val s: String)

val constLike = "a"
var mutable = "b"
val withGetter get() = "c"
val notConstInit = mutable

@Ann(constLike) // ERROR only 'const val' can be used in constant expressions
fun f1() {}

@Ann("x$constLike") // ERROR only 'const val' can be used in constant expressions
fun f2() {}

@Ann(mutable) // ERROR annotation argument must be a compile-time constant
fun f3() {}

@Ann(withGetter) // ERROR annotation argument must be a compile-time constant
fun f4() {}

@Ann(notConstInit) // ERROR annotation argument must be a compile-time constant
fun f5() {}

fun f6() {
    val local = "d"
    @Ann(local) // ERROR annotation argument must be a compile-time constant
    val x = 1
}
