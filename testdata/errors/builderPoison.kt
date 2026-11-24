class Controller<T> {
    fun yield(t: T): Boolean = true
    fun member() {}
}

fun <S> generate(g: Controller<S>.() -> Unit): S = TODO()

fun Controller<*>.foo() {}
fun <V, Y> V.bar(y: Y) {}
fun <A, B> baz(a: A, b: B) {}
fun Any.qux(y: Any) {}

// As K2, a call that can't infer its variables, of a generic extension
// on an unresolved receiver that takes the builder's this, makes the
// extension calls on the builder's receiver after it inapplicable.
fun poisoned() {
    generate { // ERROR cannot infer
        foo()
        MyUnresolvedClass().bar(this) // ERROR unresolved reference: MyUnresolvedClass
        foo() // ERROR no candidate takes the receiver
        member()
        foo() // ERROR no candidate takes the receiver
    }
}

// Other unresolved references don't.
fun notPoisoned() {
    generate { // ERROR cannot infer
        Missing() // ERROR unresolved reference: Missing
        foo()
    }
    generate { // ERROR cannot infer
        Missing().bar(1) // ERROR unresolved reference: Missing
        foo()
    }
    generate { // ERROR cannot infer
        baz(this, Missing()) // ERROR unresolved reference: Missing
        foo()
    }
    generate { // ERROR cannot infer
        Missing().qux(this) // ERROR unresolved reference: Missing
        foo()
    }
}
