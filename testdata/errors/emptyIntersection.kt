// As in K2, among several candidates, one whose variable's upper
// constraints have an empty intersection is inapplicable; a single one
// is inferred, of which kotlinc only warns.

abstract class Foo<T>
abstract class Bar<T> : Foo<T>(), Comparable<Bar<T>>

fun <T : Comparable<T>, S : T> greater(x: Bar<in S>, t: T): Int = 0
fun <T : Comparable<T>, S : T> greater(x: Bar<in S>, other: Foo<T>): String = ""

interface Expression<K>
interface Typed<K> : Expression<K>
class Column<T> : Typed<T>

infix fun <T : Comparable<T>, S : T?> Typed<in S>.less(t: T) {}
infix fun <T : Comparable<T>, S : T?> Expression<in S>.less(other: Expression<in S>) {}

open class Base
class Other

fun <T : Base> make(): T? = null

fun test(b: Bar<Long>, x: Column<Long?>, y: Double) {
    val s: String = greater(b, b)
    val o: Other? = make()
    x less y // ERROR none of the 2 candidates for 'less' takes (Double)
}
