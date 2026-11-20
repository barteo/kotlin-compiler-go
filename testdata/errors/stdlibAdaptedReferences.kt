// A reference adapted to its function type, here leaving a parameter
// to its default, isn't a KFunction.
fun withDefault(x: String = "O"): String = x
fun takesKFunction(f: kotlin.reflect.KFunction0<String>) = f

fun adapted() {
    takesKFunction(::withDefault) // ERROR adapted callable reference cannot be used against a reflection type
    val k: kotlin.reflect.KFunction0<String> = ::withDefault // ERROR adapted callable reference cannot be used against a reflection type
    val f: () -> String = ::withDefault
}
