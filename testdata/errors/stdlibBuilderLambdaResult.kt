import kotlin.experimental.ExperimentalTypeInference

// A lambda's result chooses between overloads only if its parameter has
// a type: not when it is a builder's variable that so far only other
// variables are below, as U with F of materializeFlow() below it
// (KT-81254). The ambiguity is reported, and the lambda's other
// statements still infer the variable.
interface MyFlow<E>

fun <U> lowerVar(flows: MyFlow<out U>, transform: (MutableCollection<U>) -> Unit) {}
fun <U> upperVar(flows: MyFlow<in U>, transform: (MutableCollection<U>) -> Unit) {}
fun <U> equalVar(flows: MyFlow<U>, transform: (MutableCollection<U>) -> Unit) {}
fun <U> noVar(transform: (MutableCollection<U>) -> Unit) {}
fun <U> twoLower(a: MyFlow<out U>, b: MyFlow<out U>, transform: (MutableCollection<U>) -> Unit) {}

fun <F> materializeFlow(): MyFlow<F> = TODO()

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@JvmName("myMap1")
fun <T1> MutableCollection<out T1>.myMap(transform: (T1) -> String) {}

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
@JvmName("myMap2")
fun <T2> MutableCollection<out T2>.myMap(transform: (T2) -> Int) {}

fun main() {
    lowerVar(materializeFlow()) { w ->
        w.myMap { "" } // ERROR ambiguous call
        w.add("")
    }
    equalVar(materializeFlow()) { w ->
        w.myMap { "" } // ERROR ambiguous call
        w.add("")
    }
    twoLower(materializeFlow(), materializeFlow()) { w ->
        w.myMap { "" } // ERROR ambiguous call
        w.add("")
    }
    // A type is below the variable.
    lowerVar(materializeFlow()) { w ->
        w.add("")
        w.myMap { "" }
    }
    lowerVar(materializeFlow<String>()) { w ->
        w.myMap { "" }
        w.add("")
    }
    // No variable is below it.
    upperVar(materializeFlow()) { w ->
        w.myMap { "" }
        w.add("")
    }
    noVar { w ->
        w.myMap { "" }
        w.add("")
    }
}
