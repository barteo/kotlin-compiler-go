// Syntax kotlinc's grammar accepts but reports, which kc parses as it
// does to check the rest.

object A() // ERROR constructors are not allowed for objects

fun foo<T>(x: T): T = x // ERROR type parameters must be placed before the name

fun use(y: Int) {
    try {
    } catch (val e: Exception) { // ERROR 'val' on catch parameter is not allowed
    }
    try {
    } catch (var e: Exception) { // ERROR 'var' on catch parameter is not allowed
    }
    when {
        y == 1, y == 2 -> {} // ERROR use '||' instead of commas
    }
    when (y) {
        1, 2 -> {}
    }
    foo(1)
}

interface G<T> {
    fun foo() {}
}

class CG : G<Int> {
    fun test() {
        super<G>.foo()
        super<G<Int>>.foo()
        super<G<String>>.foo() // ERROR not an immediate supertype
    }
}

fun <E> bare(y: Any) {
    class Outer<F>
    y as Outer<*>
    y as Outer // ERROR one type argument expected for
}
