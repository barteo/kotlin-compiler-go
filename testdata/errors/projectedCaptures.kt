// A receiver's out projection is K2's captured type in a lambda's
// function type: its parameters take the capture, which reads as the
// projection's type, and its result only the capture or Nothing. A
// member's type parameter bounded through the projection is bounded by
// its approximation, as an explicit type argument is.

class Out<out T> {
    fun id() = this
}
class In<in T>
class Inv<T>

class A<E> {
    fun foo(block: () -> E) {}
    fun bar(block: () -> Out<E>) {}
    fun same(f: (E) -> E) {}
    fun each(f: (E) -> Unit) {}
    fun <B : Out<E>> out(x: B) = 1
    fun <F : Inv<E>> inv(x: F) = 1
    fun <G : In<E>> into(x: G) = 1
}

fun <T> T.self(): T = this

fun test(a: A<out CharSequence>, b: A<in CharSequence>, z: Out<CharSequence>) {
    a.foo { "" } // ERROR type mismatch
    a.bar { Out<CharSequence>() } // ERROR type mismatch
    a.bar { Out() }
    a.bar { z.id() } // ERROR type mismatch
    a.same { it }
    a.same { it.self() }
    a.each { val n: Int = it.length }
    a.each { val s: CharSequence = it.self() }

    a.out(Out<CharSequence>()) // ERROR type argument is not within its bounds
    a.out(Out<Nothing>())
    a.inv(Inv<CharSequence>()) // ERROR type argument is not within its bounds
    a.into(In<CharSequence>())
    b.inv(Inv<CharSequence>()) // ERROR type argument is not within its bounds
    b.into(In<CharSequence>()) // ERROR type argument is not within its bounds
    b.into(In<Any?>())
}
