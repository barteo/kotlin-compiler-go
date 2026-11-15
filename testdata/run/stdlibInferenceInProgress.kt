// A property whose type is being inferred gives way to the functions of
// its name that its initializer calls, without asking for its own type.
import kotlin.reflect.KClass

inline fun <reified U> U.myName(): String = U::class.simpleName ?: "?"

class User<out T> {
    private val myName = myName() // the extension, on this
    fun name() = myName
}

// An extension property's getter naming another one, for a receiver it
// doesn't fit, which isn't inferred.
interface A
class B : A
open class C : A

val <T : C> KClass<T>.kind
    get() = "C"

val A.kind
    get() = when (this) {
        is B -> "B"
        is C -> this::class.kind
        else -> "?"
    }

// A delegated property named after the function of its delegate.
class Lazy {
    val lazy by lazy { 1 }
}

fun main() {
    println(User<Int>().name())
    println(B().kind + C().kind)
    println(Lazy().lazy)
}
