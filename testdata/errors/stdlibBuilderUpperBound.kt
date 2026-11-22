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

fun tkI(i: Int) {}
fun tkA(a: Any?) {}
fun Any?.ext() {}

// The lambda's parameter is of that variable's type until the call is
// inferred: it is passed on, compared, cast or given to a declaration,
// but has no members, extensions or operators, and is no argument for a
// parameter of another type than Any?.
fun parameter() {
    myCombine(materializeFlow2()) { w ->
        w.myMapWithArg(materializeFlow3(), materializeFlow4()) { x ->
            val y: Int = x
            tkA(x)
            tkA("$x")
            tkA(x == 1)
            tkA(x as Int)
            MyPair<String, Int>()
        }
        w.add(123)
    }
    myCombine(materializeFlow2()) { w ->
        w.myMapWithArg(materializeFlow3(), materializeFlow4()) { x: Int -> MyPair<String, Int>() }
        w.add(123)
    }
    myCombine(materializeFlow2()) { w ->
        w.myMapWithArg(materializeFlow3(), materializeFlow4()) { x ->
            x.toString() // ERROR the type of a receiver hasn't been inferred yet
            x.hashCode() // ERROR the type of a receiver hasn't been inferred yet
            x?.toString() // ERROR the type of a receiver hasn't been inferred yet
            x.ext() // ERROR the type of a receiver hasn't been inferred yet
            x.plus(1) // ERROR the type of a receiver hasn't been inferred yet
            x + 1 // ERROR the type of a receiver hasn't been inferred yet
            tkI(x) // ERROR argument type mismatch
            MyPair<String, Int>()
        }
        w.add(123)
    }
    myCombine(materializeFlow2()) { w ->
        w.myMapWithArg(materializeFlow3(), materializeFlow4()) {
            it.toString() // ERROR the type of a receiver hasn't been inferred yet
            MyPair<String, Int>()
        }
        w.add(123)
    }
}
