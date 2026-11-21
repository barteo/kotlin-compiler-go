import kotlin.reflect.KClass

annotation class Ann(val a: Array<String>)
annotation class K(val a: Array<KClass<*>>)

val foo = ""
const val cnst = "2"

@Ann(
    arrayOf( // ERROR only 'const val' can be used in constant expressions
        foo, // ERROR annotation argument must be a compile-time constant
        cnst
    )
)
fun test() {}

@Ann(
    arrayOf(
        cnst,
        "a"
    )
)
fun ok() {}

@K(
    arrayOf(
        ""::class, // ERROR annotation argument must be a compile-time constant
        String::class
    )
)
fun bound() {}

val kclass = String::class

@K(
    arrayOf(
        kclass, // ERROR annotation argument must be a compile-time constant
        Int::class
    )
)
fun notLiteral() {}
