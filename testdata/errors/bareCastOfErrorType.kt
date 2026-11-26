// A bare type in a cast whose subject's type is an error type has no type
// arguments to follow from it: kotlinc wants them written. A nested class
// written with its outer class's type arguments is still a type of its
// own, so a function returning it must return.

class G<T>

fun foo(p: P) { // ERROR unresolved or unsupported type: P
    val v = p as G? // ERROR one type argument expected for G; use G<*> if you don't want to pass type arguments
}

class Foo<T> {
    class Bar<U>
}

fun ex1(): Foo<String>.Bar<String> { // ERROR type arguments are only allowed on an outer class of an inner class
} // ERROR missing return statement in function 'ex1'
