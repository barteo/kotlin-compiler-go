// Generic value classes, which a program in another module uses: their
// underlying types are type parameters, which the classes' metadata
// gives, with their bounds.
package values

@JvmInline
value class A<T>(val x: T & Any)

@JvmInline
value class N<T : Number>(val n: T) {
    fun twice() = n.toDouble() * 2
}

@JvmInline
value class Box<T : Comparable<T>>(val v: T) {
    fun max(o: Box<T>) = if (v >= o.v) this else o
}

fun interface Mapper<T> {
    fun map(x: A<T & Any>): String
}
