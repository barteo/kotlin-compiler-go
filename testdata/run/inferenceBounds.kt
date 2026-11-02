// Variables inferred from upper constraints, with declared bounds the
// constraints don't satisfy: kotlinc infers their intersection.

class Foo<T>
class Bar<T> {
    fun <S : T> takeFoo(foo: Foo<in S>): String = "takeFoo"
}
class Out<out P>
interface A
open class B

fun <K : Out<L>, L : N, N : A> outs(): String {
    val foo = Foo<K>()
    return Bar<Out<B>>().takeFoo(foo)
}

interface I
interface I2 : I {
    fun func(): String
}
class C : I2 {
    override fun func() = "C.func"
}
class D : I2 {
    override fun func() = "D.func"
}

@Suppress("UNCHECKED_CAST")
fun <T : I2> materialize(): T = C() as T

fun narrowed(fail: Boolean): String {
    // T <: I and T : I2 give T = I2.
    val i: I = if (fail) D() else materialize()
    return (i as I2).func()
}

interface Base
class NotBase
fun <T, V> generic(func: V): String where T : Base, V : (T) -> Unit = "generic"

// A generic call in an if or when statement takes the other branches'
// type, which K2 infers with it.
interface Traversable {
    fun foo(): String = "Traversable.foo"
}
interface Entity<S : Entity<S>> : Traversable {
    fun <T : S> isEqualTo(): T
}
interface Path : Traversable
class EntityImpl : Entity<EntityImpl> {
    @Suppress("UNCHECKED_CAST")
    override fun <T : EntityImpl> isEqualTo(): T = this as T
}

fun statements(path: Path, e: Entity<*>, n: Int): String {
    if (n == 0) path else e.isEqualTo()
    when {
        n == 1 -> path
        else -> e.isEqualTo()
    }
    return "statements"
}

// A star argument is within a bound that mentions the class's other
// parameters: Machine<*, I>'s first is a Step<I>.
abstract class Machine<S : Machine.Step<SELF>, SELF : Machine<S, SELF>> {
    interface Step<M : Machine<out Step<M>, M>>
    interface Done<M : Machine<out Step<M>, M>> : Step<M>
}
class Finished<M : Machine<*, M>> : Machine.Done<M>
class Counter : Machine<Finished<Counter>, Counter>()

fun main() {
    println(outs<Out<Nothing>, Nothing, Nothing>())
    println(narrowed(false))
    println(generic({ _: NotBase -> }))
    println(statements(object : Path {}, EntityImpl(), 0))
    println(statements(object : Path {}, EntityImpl(), 1))
    println(Finished<Counter>() is Machine.Step<*>)
}
