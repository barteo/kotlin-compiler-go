// The type of an inner class of an inner class, written with the outer
// class's type arguments: Foo<String>.Inner.Deeper<Int>.
class Foo<T>(val t: T) {
    inner class Inner(val p: T) {
        inner class Deeper<K>(val k: K) {
            fun show() = "$t $p $k"
        }
    }

    typealias ToDeeper<L> = Foo<String>.Inner.Deeper<L>
}

fun show(d: Foo<String>.Inner.Deeper<Int>) = d.show()

fun main() {
    val d: Foo<String>.Inner.Deeper<Int> = Foo("a").Inner("b").Deeper(1)
    println(show(d))
    val e: Foo.ToDeeper<Char> = Foo("c").Inner("d").Deeper('e')
    println(e.show())
}
