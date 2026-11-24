import p.*

// A type of a class whose name another module's class has too, with that
// module's type arguments, is a type of neither, as in K2.
fun test() {
    foo(M1().a) // ERROR type mismatch: inferred type is A<Int> but A was expected
    ia(M1().b) // ERROR type mismatch: inferred type is B but IA was expected
    c(M1().b) // ERROR type mismatch: inferred type is B but C<Int> was expected
    d(M1().b)
    // But the type of a class without type parameters is one of its name
    // with any arguments, as K2 sees it, if not through a supertype.
    f(M1().f)
}

// Both Bases are the first module's: its foo has no bound. A class-path
// member's type parameters' bounds count, as the program's do.
class Foo : BaseA, BaseB {
    override fun <T> foo(t: Array<T>) {}
    override fun <T : Base> foo(t: Array<T>) {} // ERROR 'foo' overrides nothing
}
