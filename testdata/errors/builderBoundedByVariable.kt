// A builder's type variable bounded by another variable of the call that
// has constraints of its own isn't inferred from the lambda: as in
// kotlinc, the other is fixed first and bounds it by a proper type.
class Container<T> { fun produce(): T = null!!; fun put(x: T) {} }
fun <T> consume(arg: T) {}
open class Parent
open class Child : Parent()
fun <A, B : A> two(a: A, lambda: (Container<B>) -> Unit): B = null!!
fun <A, B : A, C : B> trans(a: A, lambda: (Container<C>) -> Unit): C = null!!
fun <A, B : A> recv(a: Container<A>, lambda: Container<B>.() -> Unit): B = null!!
fun <B : CharSequence> one(lambda: (Container<B>) -> Unit): B = null!!

fun test(cp: Container<Parent>) {
    val d = two(Parent()) { it.put(Child()) }
    val d1: Int = d // ERROR type mismatch: inferred type is Parent but Int was expected
    two(Parent()) {
        consume<Child>(it.produce()) // ERROR type mismatch: inferred type is Parent but Child was expected
    }
    val t = trans(Parent()) { it.put(Child()) }
    val t1: Int = t // ERROR type mismatch: inferred type is Parent but Int was expected
    val r = recv(cp) { put(Child()) }
    val r1: Int = r // ERROR type mismatch: inferred type is Parent but Int was expected
    // A declared bound alone leaves the variable to the lambda.
    val o = one { it.put("") }
    val o1: Int = o // ERROR type mismatch: inferred type is String but Int was expected
}
