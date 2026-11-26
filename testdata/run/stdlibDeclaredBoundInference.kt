// A type parameter's declared bound constrains the type arguments of an
// argument that are still to be inferred, as K2's declared upper bound
// constraint: E : Box<String> gives box()'s T String.
open class Box<out T>(val items: List<T>)

fun <T> box(): Box<T> = Box(emptyList())
fun <T> none(): List<T> = emptyList()

fun <E : Box<String>> describe(b: E): String = "box of " + b.items.size
fun <F : List<CharSequence>> count(l: F): Int = l.size

class Holder<T> {
    fun <G : Box<T>> take(b: G): Int = b.items.size
}

fun main() {
    println(describe(box()))
    println(count(none()))
    println(Holder<Int>().take(box()))
    val h: Holder<out Number> = Holder<Int>()
    println(h.take(box()))
}
