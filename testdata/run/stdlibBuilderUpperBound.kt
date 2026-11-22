import kotlin.experimental.ExperimentalTypeInference

// A lambda's result chooses the overload in a builder's lambda, where
// the type of the lambda's parameter is a variable with only variables
// around it; the builder's variable is inferred from what is chosen.
class MyFlow<E>
class MyPair<R1, R2>

fun <U> myCombine(flows: MyFlow<out U>, transform: (MutableCollection<U>) -> Unit): MutableCollection<U> {
    val c = mutableListOf<U>()
    transform(c)
    return c
}

fun <F> materializeFlow(): MyFlow<F> = MyFlow()

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@JvmName("myMapWithArg1")
fun <T1> MutableCollection<out T1>.myMapWithArg(v1: MyFlow<out T1>, v2: MyFlow<in T1>, transform: (T1) -> MyPair<String, in T1>) {
    println("String")
}

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@JvmName("myMapWithArg2")
fun <T2> MutableCollection<out T2>.myMapWithArg(v1: MyFlow<out T2>, v2: MyFlow<in T2>, transform: (T2) -> MyPair<Int, in T2>) {
    println("Int")
}

fun main() {
    val a = myCombine(materializeFlow()) { w ->
        w.myMapWithArg(materializeFlow(), materializeFlow()) { MyPair<String, Int>() }
        w.add(123)
    }
    println(a)
    val b = myCombine(materializeFlow()) { w ->
        w.myMapWithArg(materializeFlow(), materializeFlow()) { MyPair<Int, Int>() }
        w.add(4)
        w.add(5)
    }
    println(b.sum())
}
