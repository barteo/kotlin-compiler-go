// A suspend function type passed for a reified type parameter that reaches
// typeOf, through the program's other inline functions too, has no KType.
import kotlin.reflect.typeOf

inline fun <reified X> f() = g<List<X>>()
inline fun <reified Y> g() = typeOf<Y>()
inline fun <reified Z> h() = Z::class.java.name

fun test() {
    f<suspend (String) -> Unit>() // ERROR suspend function types can't be used in 'typeOf'
    g<suspend () -> Int>() // ERROR suspend function types can't be used in 'typeOf'
    f<() -> Unit>()
    h<suspend () -> Int>()
}
