// A lambda that a builder's lambda returns with return@label is checked
// as one it ends with: in PCLA mode, at once, with its parameter's variable
// a builder's variable too; with the variables known, after they are fixed.
class Container<T> {
    fun produce(): T = null!!
}

class TypePair<L, R>

fun <A, B : A> func(lambda: (Container<A>) -> (Container<B>) -> Unit): TypePair<A, B> = null!!

fun <T> consume(arg: T) {}

open class Parent
open class Child : Parent()

fun pclaMode() {
    val pair: TypePair<Parent, Child> = func { containerA ->
        consume<Parent>(containerA.produce())
        return@func { containerB ->
            consume<Child>(containerB.produce())
        }
    }
}

fun standardMode() {
    func<Parent, _> { containerA ->
        consume<Parent>(containerA.produce())
        return@func { containerB ->
            consume<Child>(containerB.produce()) // ERROR type mismatch: inferred type is Parent but Child was expected
        }
    }
}
