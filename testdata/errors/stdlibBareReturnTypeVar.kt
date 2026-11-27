// A bare return@l is fine in a lambda whose result is the callee's type
// parameter, even one the call gives explicitly, but not in one whose
// expected type is () -> Any.
fun <R> call(f: () -> R): R = f()

class Box<T> {
    fun get(f: () -> T): T = f()
}

fun test(c: Boolean, b: Box<Any>) {
    run<Any> l@ {
        if (c) return@l
        ""
    }
    call<Any> l@ {
        if (c) return@l
        return@l
    }
    b.get l@ {
        if (c) return@l // ERROR this lambda must return a value of type Any
        ""
    }
}

val v: () -> Any = l@ {
    if ("0".hashCode() == 42) return@l // ERROR this lambda must return a value of type Any
    ""
}
