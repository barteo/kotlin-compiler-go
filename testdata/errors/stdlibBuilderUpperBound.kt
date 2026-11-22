import kotlin.experimental.ExperimentalTypeInference

// In a builder's lambda, a lambda's result chooses between overloads
// whose parameter's type is a variable with several variables below it
// and one above: as in kotlinc (KT-81254), the lambda is analysed with
// the one above, and what it chooses then infers the builder's variable.
interface MyFlow<E>
class MyPair<R1, R2>

fun <U> myCombine(flows: MyFlow<out U>, transform: (MutableCollection<U>) -> Unit) {}

fun <F2> materializeFlow2(): MyFlow<F2> = TODO()
fun <F3> materializeFlow3(): MyFlow<F3> = TODO()
fun <F4> materializeFlow4(): MyFlow<F4> = TODO()

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@JvmName("myMapWithArg1")
fun <T1> MutableCollection<out T1>.myMapWithArg(v1: MyFlow<out T1>, v2: MyFlow<in T1>, transform: (T1) -> MyPair<String, in T1>) {}

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@JvmName("myMapWithArg2")
fun <T2> MutableCollection<out T2>.myMapWithArg(v1: MyFlow<out T2>, v2: MyFlow<in T2>, transform: (T2) -> MyPair<Int, in T2>) {}

fun main() {
    myCombine(materializeFlow2()) { w ->
        w.myMapWithArg(materializeFlow3(), materializeFlow4()) { MyPair<String, Int>() }
        w.add(123)
    }
    myCombine(materializeFlow2()) { w ->
        w.myMapWithArg(materializeFlow3(), materializeFlow4()) { MyPair<String, Int>() }
        w.add("") // ERROR type mismatch: inferred type is String but Int was expected
    }
}
