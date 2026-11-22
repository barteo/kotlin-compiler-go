fun <R> scope(block: () -> R): R = block()
fun take(i: Int, f: () -> Int) {}
fun plain(i: Int) {}
fun <X> id(x: X): X = x
fun <X> gen(x: X, f: () -> Unit): X = x
fun String.sfoo(f: () -> Unit) {}

// As K2, a generic extension called on an unresolved receiver with a
// lambda fails the constraint system that the builder's lambda shares:
// every call after it that takes a lambda, or has a type with the
// builder's variable, is inapplicable.
fun failed(k: Int) {
    sequence { // ERROR cannot infer type for type parameter 'T'
        val r = Unres(1).apply { } // ERROR cannot infer type for receiver parameter
        take(k) { 1 } // ERROR inferred type is () -> Int but () -> Int was expected
        plain(k)
        val s = scope { 1 } // ERROR inferred type is () -> Int but () -> R was expected
        plain(s) // ERROR inferred type is R but Int was expected
        val t = id(k)
        plain(t)
        yield(1) // ERROR inferred type is Int but T was expected
        yield(Unres2()) // ERROR inapplicable candidate 'yield'
        listOf(1).map { it } // ERROR candidate 'map' is inapplicable because of a receiver type mismatch
    }
}

fun failedLet(k: Int) {
    buildList { // ERROR cannot infer type for type parameter 'E'
        Unres(1).let { 1 } // ERROR cannot infer type for type parameter 'T'
        add(1) // ERROR inferred type is Int but E was expected
        take(k) { 1 } // ERROR inferred type is () -> Int but () -> Int was expected
    }
}

// Other calls with an unresolved reference don't.
fun notFailed(k: Int) {
    sequence {
        Unres(1) // ERROR unresolved reference: Unres
        take(k) { 1 }
        yield(1)
    }
    sequence {
        Unres(1).sfoo { } // ERROR unresolved reference: Unres
        take(k) { 1 }
        yield(1)
    }
    sequence {
        gen(Unres(1)) { } // ERROR cannot infer type for type parameter 'X'
        take(k) { 1 }
        yield(1)
    }
}
