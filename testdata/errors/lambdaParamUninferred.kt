// A lambda parameter declared without a type that can't be inferred is
// reported with the call's variable it leaves uninferred, as kotlinc does.
fun <T, R> foo(block: (T) -> R) = block
fun <T> callParam(arg: T) {}
fun callAny(arg: Any?) {}
fun <T> takeInt(block: (T) -> Int) {}

fun test() {
    foo { // ERROR cannot infer type for type parameter 'T'
        x -> // ERROR cannot infer type for type parameter 'T'
        43
    }
    callParam { // ERROR cannot infer type for type parameter 'T'
        p -> // ERROR cannot infer
        p
    }
    callAny {
        e -> // ERROR cannot infer
        e
    }
    callParam { 1 }
    foo { x: String -> x.length }
    takeInt<String> { s -> s.length }
    callParam(::callAny)
    foo(::callAny)
}
