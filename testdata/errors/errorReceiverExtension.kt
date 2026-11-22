// A call on a receiver whose type has an error is, as in kotlinc, a call
// of the only extension function of its name: the type parameters that
// the receiver would give a type can't be inferred. A name that several
// extensions have, or none, is left alone, and so is an extension that
// isn't generic.
interface MyFlow<out R>
class Other<A>
class C { fun member() {} }

fun <X, Y> MyFlow<X>.fm(transform: (value: X) -> MyFlow<Y>): MyFlow<Y> = null!!
fun <X> MyFlow<X>.one() {}
fun <X> MyFlow<X>.over(a: Int) {}
fun <X> Other<X>.over(a: Int) {}
fun String.plainExt() {}
fun <T> T.gen() {}

val x = nonExistingFunction() // ERROR unresolved reference: nonExistingFunction

fun main() {
    x.fm { y -> y } // ERROR cannot infer type for type parameter 'X'
    x.one() // ERROR cannot infer type for type parameter 'X'
    x.gen() // ERROR cannot infer type for type parameter 'T'
    val z = x.one() // ERROR cannot infer type for type parameter 'X'
    x.over(1)
    x.plainExt()
    x.member()
    x.nope()
    x?.one() // ERROR cannot infer type for type parameter 'X'
}
