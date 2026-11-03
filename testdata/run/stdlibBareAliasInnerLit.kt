// A typealias written bare in is and as, and class literals of inner
// classes of generic classes.

typealias L<T> = List<T>
typealias Z<U> = B<U>

sealed class C<out T, out U>
class A<out T>(val x: T) : C<T, Nothing>()
class B<out U>(val x: U) : C<Nothing, U>()

class Generic<T> { inner class Inner }
class Outer<T> { inner class Inner<S> }

fun bar(x: String): C<Int, String> = B(x)

fun main() {
    val c: Collection<Int> = listOf(1, 2, 3)
    println(c is L)
    val l = c as L
    println(l[1])
    when (val x = bar("ok")) {
        is Z -> println(x.x.length)
        else -> println("else")
    }
    println(Generic.Inner::class == Generic<Int>().Inner()::class)
    println(Outer.Inner::class == Outer<String>().Inner<Int>()::class)
    val i: Outer<String>.Inner<Int> = Outer<String>().Inner<Int>()
    println(i::class == Outer.Inner::class)
}
