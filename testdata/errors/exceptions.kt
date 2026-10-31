class Box(val n: Int)

fun throwInt(): Nothing = throw 1 // ERROR inferred type is Int but Throwable was expected

fun catchString(): Int = try {
    1
} catch (e: String) { // ERROR the type of a catch parameter must be a subtype of Throwable
    2
}

// An enum entry's object is on the stack while its arguments run.
enum class Entry(val n: Int) {
    A(try { 1 } catch (e: Exception) { 2 }) // ERROR a try expression can't be an argument of a constructor call
}

fun mismatch(): Int = try {
    1
} catch (e: Exception) { // ERROR type mismatch
    "two"
}

fun wrongArgs() {
    require() // ERROR no function 'require' takes 0 arguments
    error("a", "b") // ERROR no function 'error' takes 2 arguments
}
