// is and as on read-only Kotlin collections that are mutable Java ones,
// the program's contains on a range over the standard library's, a
// value class an override returns boxed where the overridden function
// returns Any or T, and references of function types that equal each
// other, as FunctionReferences, adapted ones too.

class Itr : Iterator<String> by ArrayList<String>().iterator()
class Lst : List<String> by ArrayList<String>()

inline fun <reified T> isA(x: Any) = x is T

operator fun ClosedRange<Int>.contains(value: Long) = false

@JvmInline
value class X(val x: Any)

interface Foo {
    fun foo(): Any
}

interface Gen<T> {
    fun get(): T
}

interface NullableX {
    fun x(): X?
}

class Impl : Foo, Gen<X>, NullableX {
    override fun foo(): X = X("foo")
    override fun get(): X = X("get")
    override fun x(): X = X("x")
}

class V {
    fun target(a: String = "a", b: String = "b"): String = a + b
    fun many(vararg s: String): String = s.joinToString()
}

fun top(): Int = 1

fun two(f: (V, String, String) -> String): Any = f
fun one(f: (V, String) -> String): Any = f
fun unit(f: (V, String, String) -> Unit): Any = f
fun elems(f: (V, String, String) -> String): Any = f
fun array(f: (V, Array<String>) -> String): Any = f
fun topF(f: () -> Int): Any = f
fun topUnit(f: () -> Unit): Any = f

fun main() {
    val itr: Any = Itr()
    val lst: Any = Lst()
    println("${itr is MutableIterator<*>} ${itr is Iterator<*>} ${lst is MutableList<*>} ${ArrayList<String>() is MutableList<*>}")
    println("${itr as? MutableIterator<*>} ${isA<MutableCollection<*>>(lst)} ${isA<Collection<*>>(lst)}")
    try {
        lst as MutableList<*>
    } catch (e: ClassCastException) {
        println("ClassCastException")
    }

    println("${10L in 1..10} ${10 in 1..10}")

    val impl = Impl()
    val foo: Foo = impl
    val gen: Gen<X> = impl
    val nx: NullableX = impl
    val any: Any = foo.foo()
    println("${any is X} ${gen.get().x} ${nx.x()?.x} ${impl.foo().x}")

    val v = V()
    println(two(V::target) == two(V::target))
    println(one(V::target) == one(V::target))
    println(one(V::target) == two(V::target))
    println(unit(V::target) == unit(V::target))
    println(unit(V::target) == two(V::target))
    println(elems(V::many) == elems(V::many))
    println(array(V::many) == array(V::many))
    println(topF(::top) == topF(::top))
    println(topUnit(::top) == topF(::top))
    println(two(V::target).hashCode() == two(V::target).hashCode())
    val f = one(V::target) as (V, String) -> String
    println(f(v, "x"))
}
