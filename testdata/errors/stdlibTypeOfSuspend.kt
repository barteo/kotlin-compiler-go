// kotlin.reflect has no KType of a suspend function type: kotlinc's JVM
// backend reports typeOf of one.
import kotlin.reflect.typeOf

fun types() {
    typeOf<suspend () -> Int>() // ERROR suspend function types can't be used in 'typeOf'
    typeOf<List<suspend Int.() -> Unit>>() // ERROR suspend function types can't be used in 'typeOf'
    typeOf<() -> Int>()
    typeOf<List<*>>()
}
