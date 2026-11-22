// A lambda returned with return@label is the lambda's result as one it
// ends with is: its parameter types come from the result type expected,
// once the call's type arguments are known, and its body is checked
// with them.
class Container<T> {
    fun produce(): T = null!!
}

class TypePair<L, R>

fun <A, B : A> func(lambda: (Container<A>) -> (Container<B>) -> Unit): TypePair<A, B> = null!!

fun <T> consume(arg: T) {}

open class Parent
open class Child : Parent()

fun inferred(): TypePair<Parent, Child> = func { a ->
    consume<Parent>(a.produce())
    return@func { b ->
        consume<Child>(b.produce())
    }
}

fun given() {
    func<Parent, _> { a ->
        consume<Parent>(a.produce())
        return@func { b ->
            consume<Child>(b.produce()) // ERROR type mismatch
        }
    }
}

fun tail() {
    func<Parent, _> { a ->
        consume<Parent>(a.produce());
        { b ->
            consume<Child>(b.produce()) // ERROR type mismatch
        }
    }
}
