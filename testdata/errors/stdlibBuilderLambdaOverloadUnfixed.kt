// A lambda can't choose between overloads by its result where its
// parameter's variable has only builder's variables below it, more than
// one, which give no type yet: K2 can't fix it to their common supertype
// for the lambda. With one, K2 semi-fixes it to it.
interface MyFlow<E>

fun <U> myCombine(flows: MyFlow<out U>, transform: (MutableCollection<U>) -> Unit) {}
fun <F> materializeFlow(): MyFlow<F> = TODO()

@OptIn(kotlin.experimental.ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@kotlin.jvm.JvmName("myMap1")
fun <T1> MutableCollection<out T1>.myMap(transform: (T1) -> String) {}

@OptIn(kotlin.experimental.ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@kotlin.jvm.JvmName("myMap2")
fun <T2> MutableCollection<out T2>.myMap(transform: (T2) -> Int) {}

fun main() {
    myCombine(materializeFlow()) { w ->
        w.add("")
        w.myMap { "" }
    }
    myCombine(materializeFlow()) { w ->
        w.myMap { "" } // ERROR ambiguous call: 2 candidates for 'myMap'
        w.add("")
    }
    buildList {
        flatMap { listOf(it) }
        add("")
    }
}
