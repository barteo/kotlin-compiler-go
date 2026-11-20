class Foo

fun f(): Int = 1

fun elvis(s: String?, n: String) {
    val a = null ?: n // USELESS_ELVIS_LEFT_IS_NULL
    val b = s ?: null // USELESS_ELVIS_RIGHT_IS_NULL
    val c = n ?: null // USELESS_ELVIS
    val d = s ?: n
    println(a); println(b); println(c); println(d)
}

fun assertions() {
    val a = { 1 }!! // NOT_NULL_ASSERTION_ON_LAMBDA_EXPRESSION
    val b = ::f!! // NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE
    val c = String::length!! // NOT_NULL_ASSERTION_ON_CALLABLE_REFERENCE
    val d = (fun() = 1)!!
    val e = Foo::class
    println(a); println(b); println(c); println(d); println(e)
}
