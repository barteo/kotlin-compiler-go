// A reference that may name several functions is still ambiguous where
// the function types around it take other inputs, whose common supertype
// both functions take, and still needs a function that fits the KFunction
// type that explicit type arguments give.
import kotlin.reflect.KFunction2

interface A
interface B

fun foo(x: A) {}
fun foo(x: B) {}
fun <T> id(x: T) = x
fun <T> select(vararg x: T) = x[0]

fun resolve(a: Number, b: Int) = ""
fun resolve(a: Number, b: String) = ""
fun <K> kfun(f: KFunction2<K, Double, String>) = f
fun <K> outKfun(f: KFunction2<out K, String, String>) = f // ERROR projection is conflicting

fun test() {
    select(id(::foo), id { x: A -> }, id { x: B -> }) // ERROR overload resolution ambiguity
    kfun<Number>(::resolve) // ERROR none of the 2 candidates
    outKfun<Number>(::resolve) // ERROR overload resolution ambiguity
}
