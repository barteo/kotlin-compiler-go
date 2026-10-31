class A {
    lateinit val a: String // ERROR 'lateinit' modifier is allowed only on mutable properties
    lateinit var b: String = "" // ERROR 'lateinit' modifier is not allowed on properties with an initializer
    lateinit var c: String? // ERROR 'lateinit' modifier is not allowed on properties of nullable types
    lateinit var d: Int // ERROR 'lateinit' modifier is not allowed on properties of primitive types
    lateinit var e: String // ERROR 'lateinit' modifier is not allowed on properties with a custom getter or setter, or delegated ones
        get() = "e"
    var f = 1
    fun g() = this::f.isInitialized // ERROR this declaration can only be called on a reference to a lateinit property
}

fun h() {
    lateinit var x: Int // ERROR 'lateinit' modifier is not allowed on properties of primitive types
}

fun outside(a: A) = a::b.isInitialized // ERROR backing field of 'var b: String' is not accessible at this point

open class Open {
    open tailrec fun f(n: Int): Int = if (n == 0) 0 else f(n - 1) // ERROR tailrec is not allowed on open members
}
