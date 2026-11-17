// A lambda or reference passed for a type variable has a result of its
// own, which the variable's other bounds don't fix before it is checked.
interface A { val name: String }
class B : A { override val name = "B" }
class C : A { override val name = "C" }

fun <K> select(x: K, y: K): K = y
fun <T> id(x: T): T = x

interface Conv {
    fun toB(): B
    fun toC(): C
    fun toC(n: Int): C
}

object Impl : Conv {
    override fun toB() = B()
    override fun toC() = C()
    override fun toC(n: Int) = C()
}

// Each lambda declares its parameter's type: K is (Conv) -> A.
fun typedParams(): (Conv) -> A = select({ c: Conv -> c.toB() }, { c: Conv -> c.toC() })

// The second id's lambda takes no input from the first's () -> B.
fun nested() = select(id { B() }, id { C() })

// It takes the receiver, though: Int.() -> R.
fun receiver() = select(id { this + 1 }, fun Int.() = this * 2)

// R is A, from acc's type, not the B of the initial value.
fun folded(xs: List<String>) = xs.fold(B()) { acc: A, s -> if (s == "c") C() else acc }

// toC is toC(), which takes the inputs toB does.
fun refs() = select(Conv::toB, Conv::toC)

fun main() {
    println(typedParams()(Impl).name)
    println(nested()().name)
    println(receiver()(20))
    println(folded(listOf("a", "c")).name)
    println(folded(listOf("a")).name)
    println(refs()(Impl).name)
}
