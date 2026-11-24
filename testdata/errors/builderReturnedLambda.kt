// A lambda returned with return@label at the end of a builder's lambda
// takes its parameter types as one it ends with does.
class Container<T> { fun produce(): T = null!! }
class TypePair<L, R>
fun <A, B : A> func(lambda: (Container<A>) -> (Container<B>) -> Unit): TypePair<A, B> = null!!
fun <T> consume(arg: T) {}
open class Parent
open class Child : Parent()

fun test() {
    val a = func { ca ->
        consume<Parent>(ca.produce())
        return@func { cb -> consume<Child>(cb.produce()) }
    }
    val a1: Int = a // ERROR type mismatch: inferred type is TypePair<Parent, Child> but Int was expected
    val b = func<Parent, _> { ca ->
        return@func { cb ->
            consume<Child>(cb.produce()) // ERROR type mismatch: inferred type is Parent but Child was expected
        }
    }
    val b1: Int = b // ERROR type mismatch: inferred type is TypePair<Parent, Parent> but Int was expected
}
