inline fun <reified T> foo() {}

typealias TA<T, R> = Array<R>

class B<T>(val array: Array<T>)

fun <T> bar() {
    foo<Array<T>>() // ERROR cannot use an array of type parameter 'T' as a reified type argument
    foo<Array<Array<T>>>() // ERROR cannot use an array of type parameter 'T' as a reified type argument
    foo<TA<String, T>>() // ERROR cannot use an array of type parameter 'T' as a reified type argument
    foo<TA<T, String>>()
    foo<Array<Int>>()
    foo<List<Array<T>>>()
    B<Array<T>>(arrayOf()) // ERROR cannot use an array of type parameter 'T' as a reified type argument
}

inline fun <reified R> ok() = foo<Array<R>>()

fun <T> fail(block: () -> T): Pair<Array<T>, Array<T>> = Pair(arrayOf( // ERROR cannot use 'T' as reified type parameter
    block()), arrayOf()) // ERROR cannot use 'T' as reified type parameter

inline fun <reified T> fine(block: () -> T): Pair<Array<T>, Array<T>> = Pair(arrayOf(block()), arrayOf())
