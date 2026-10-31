// Errors of the standard library's intrinsics that kc builds in.

class NotEnum

fun <T> plain(): Array<T> = enumValues<T>() // ERROR cannot use 'T' as reified type parameter

fun main() {
    enumValues<NotEnum>() // ERROR is not an enum class
    println(Int.MAX_VALUE)
}

// Not exhaustive, so a statement: the lambda returns Unit.
fun soft(xs: List<Int>): List<String> = xs.map { x -> when (x) { 1 -> "one" } } // ERROR inferred type is List<Unit> but List<String> was expected

fun noInfo(b: Boolean) = if (b) listOf() else emptyList() // ERROR cannot infer type for type parameter

// With an expected Any, a branch takes the other branches' type, which an
// Int isn't.
fun anyInfo(b: Boolean): Any = if (b) emptyList() else 3 // ERROR cannot infer type for type parameter

// A star projection the expected type has is no capture to infer from.
fun starInfo(): List<*> = emptyList() // ERROR cannot infer type for type parameter

// Without @Suppress("INVISIBLE_REFERENCE"), the class path's internal
// classes are not visible.
abstract class Machine : kotlin.coroutines.jvm.internal.ContinuationImpl(null) // ERROR class ContinuationImpl is not public

// A KFunction is a function type, but a function type isn't a KFunction.
fun notKFun(f: (Int) -> Int): kotlin.reflect.KFunction1<Int, Int> = f // ERROR type mismatch: inferred type is (Int) -> Int but KFunction1<Int, Int> was expected

// Through a projection, a lambda's parameter is Kotlin's captured type,
// which reads as the projection's type or the bound: not Nothing.
class Holder<T>(val t: T) {
    fun use(f: (T) -> Unit) = f(t)
}

fun projectedLambdaParams(f: java.util.concurrent.CompletableFuture<*>, h: Holder<*>, c: MutableList<java.util.function.Consumer<out CharSequence>>) {
    f.handle { v, _ -> val n: Int = v } // ERROR type mismatch: inferred type is Any? but Int was expected
    h.use { v -> val n: Int = v } // ERROR type mismatch: inferred type is Any? but Int was expected
    c.add { v -> val n: Int = v } // ERROR type mismatch: inferred type is CharSequence! but Int was expected
}

fun optionalMap(o: java.util.Optional<out CharSequence>) {
    o.map { it.length }
    o.map { v -> val n: Int = v } // ERROR type mismatch: inferred type is CharSequence! but Int was expected
}
