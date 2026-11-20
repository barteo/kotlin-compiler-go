// As K2, the common supertype of instances of a class whose arguments
// differ only in their flexibility, Inv<J> and Inv<J!>, is the first: a
// type argument that is a platform type stays one only where it comes
// first.

class Inv<T>
interface Out<out T>
fun <T> f(a: Out<T>, b: Out<T>, c: Out<T>): T = null!!
fun <T> f2(a: Out<T>, b: Out<T>): T = null!!
fun <T> out(t: T): Out<Inv<T>> = null!!

fun test(a: Out<Nothing>, b: Out<Inv<J>>) {
    val o = out(J.j())
    val v1: Inv<J?> = f(a, b, out(J.j())) // ERROR type mismatch: inferred type is Inv<J> but Inv<J?> was expected
    val v2: Inv<J?> = f2(b, o) // ERROR type mismatch: inferred type is Inv<J> but Inv<J?> was expected
    val v3 = f2(o, b)
    val n3: Inv<J?> = v3
    val n4: Inv<J> = v3
    val v5 = f(a, o, b)
    val n5: Inv<J?> = v5
}
