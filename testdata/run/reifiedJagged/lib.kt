// Array(n) { init } in an inline function, whose init invokes the
// function's lambda parameter: the lambda is inlined into the array's
// loop, as kotlinc inlines the Array constructor.

inline fun <reified T> jagged(x: (Int, Int) -> T): Array<Array<T>> = Array(2) { i ->
    Array(2) { j -> x(i, j) }
}

inline fun <reified T> deep(x: (Int, Int, Int) -> T): Array<Array<Array<T>>> = Array(1) { i ->
    Array(1) { j -> Array(2) { k -> x(i, j, k) } }
}

inline fun squares(x: (Int) -> Int): IntArray = IntArray(4) { i ->
    if (i == 2) return@IntArray -1
    x(i * i)
}

inline fun <reified T> counted(x: (Int) -> T): Array<T> {
    var n = 0
    val a = Array(3) { n++; x(it) }
    println("counted $n")
    return a
}
