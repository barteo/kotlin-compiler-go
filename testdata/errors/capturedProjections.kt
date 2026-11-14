// A projection captured by a variable: a supertype's parameter takes
// it, a reified parameter doesn't, and a star of a covariant parameter
// is its bound, so a member written to through it takes the bound.

interface Fu<V>
open class CF<T : Number> : Fu<T>
fun <F> wait(f: Fu<F>): F = TODO()
fun waitStar(x: CF<*>): Number = wait(x)

class A<F>
inline fun <reified T> reified(x: A<T>) {}
fun captured(x: A<out CharSequence>) {
    reified(x) // ERROR cannot use captured type as reified type parameter
}

data class Out<out T>(val prop: T)
fun copyStar(b: Out<*>) = b.copy("")

class B<T1> {
    fun <X1 : C<T1>> m(): Int = 1
}
class C<T>
fun bound(b: B<out Number>) {
    b.m<C<out Number>>() // ERROR type argument is not within its bounds
}
